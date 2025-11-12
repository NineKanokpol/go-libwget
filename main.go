package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"golang.org/x/term"
)

var version = "dev"

type Config struct {
	Endpoint  string `json:"endpoint"` // e.g. s3.amazonaws.com or play.min.io:9000
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
	Region    string `json:"region"` // e.g. ap-southeast-1 (เว้นว่างได้ถ้า MinIO ไม่บังคับ)
	UseSSL    bool   `json:"useSSL"` // true=https
	Bucket    string `json:"bucket"` // ออปชัน: บัคเก็ตที่อยากทดสอบเข้าถึง
}

func main() {
	// subcommand: connect (มี --use-config / --save)
	connectCmd := flag.NewFlagSet("connect", flag.ExitOnError)
	useConfig := connectCmd.Bool("use-config", false, "use saved config at ~/.mycli/config.json")
	saveConfig := connectCmd.Bool("save", false, "save answers to ~/.mycli/config.json after successful connect")

	// mycli -version
	showVersion := flag.Bool("version", false, "print version and exit")

	if len(os.Args) == 1 {
		usage()
		return
	}

	switch os.Args[1] {
	case "-version", "--version":
		*showVersion = true
	}

	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}

	switch os.Args[1] {
	case "connect":
		connectCmd.Parse(os.Args[2:])
		if *useConfig {
			cfg, err := loadConfig()
			if err != nil {
				fmt.Println("failed to load config:", err)
				os.Exit(1)
			}
			if err := doConnect(cfg); err != nil {
				fmt.Println("connect error:", err)
				os.Exit(1)
			}
			fmt.Println("✅ Connected successfully (using saved config)")
			return
		}

		// wizard: ถามค่าตามลำดับเหมือน login
		cfg, err := wizard()
		if err != nil {
			fmt.Println("input error:", err)
			os.Exit(1)
		}

		if err := doConnect(cfg); err != nil {
			fmt.Println("connect error:", err)
			os.Exit(1)
		}
		fmt.Println("✅ Connected successfully")

		if *saveConfig {
			if err := saveConfigFile(cfg); err != nil {
				fmt.Println("warn: cannot save config:", err)
			} else {
				fmt.Println("💾 Saved config at", configPath())
			}
		}
	default:
		usage()
	}
}

func usage() {
	fmt.Println(`mycli - simple S3/MinIO connect wizard

Usage:
  mycli -version
  mycli connect                # เปิดตัวช่วยถามค่าทีละข้อ แล้วพยายามเชื่อมต่อ
  mycli connect --save         # ถามค่า → ต่อสำเร็จ → บันทึก ~/.mycli/config.json
  mycli connect --use-config   # ใช้ค่าจาก ~/.mycli/config.json แล้วเชื่อมต่อทันที

Tips:
  - Endpoint: เช่น "s3.amazonaws.com" หรือ "minio.yourdomain.com:9000"
  - Region: สำหรับ AWS S3 ต้องกรอก เช่น "ap-southeast-1"; MinIO ส่วนมากเว้นว่างได้
  - UseSSL: พิมพ์ y/n (y = https)
`)
}

func wizard() (Config, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("=== S3/MinIO Connect Wizard ===")
	endpoint := mustReadLine(reader, "Endpoint (e.g. s3.amazonaws.com or minio.example.com:9000): ")
	accessKey := mustReadLine(reader, "Access Key: ")
	secretKey := mustReadPassword("Secret Key (input hidden): ")
	region := readLine(reader, "Region (leave empty if not required): ")
	useSSL := mustReadYesNo(reader, "Use SSL? [y/N]: ")
	bucket := readLine(reader, "Bucket to test access (optional): ")

	cfg := Config{
		Endpoint:  endpoint,
		AccessKey: accessKey,
		SecretKey: secretKey,
		Region:    region,
		UseSSL:    useSSL,
		Bucket:    bucket,
	}
	return cfg, nil
}

func doConnect(cfg Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// ✅ สำคัญ: endpoint = "s3gw.inet.co.th:8082" (ไม่มี http://)
	endpoint := strings.TrimSpace(cfg.Endpoint)

	opts := &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure:       cfg.UseSSL,
		BucketLookup: minio.BucketLookupPath,
	}
	if region := strings.TrimSpace(cfg.Region); region != "" {
		opts.Region = region
	}

	client, err := minio.New(endpoint, opts)
	if err != nil {
		return fmt.Errorf("init client: %w", err)
	}

	// ถ้ากรอก bucket มาก็เช็คเฉพาะ bucket ก่อน (แน่นอนกว่า)
	if b := strings.TrimSpace(cfg.Bucket); b != "" {
		exists, err := client.BucketExists(ctx, b)
		if err != nil {
			return fmt.Errorf("check bucket: %w", err)
		}
		if !exists {
			fmt.Println("⚠ bucket not accessible:", b)
		} else {
			fmt.Println("✔ bucket accessible:", b)
		}
	}

	// ✅ ทดสอบ ListBuckets (ต้องมีสิทธิ์ s3:ListAllMyBuckets)
	bs, err := client.ListBuckets(ctx)
	if err != nil {
		return fmt.Errorf("list buckets failed: %w", err)
	}
	fmt.Println("✔ buckets:", len(bs))
	for _, b := range bs {
		fmt.Println("  -", b.Name)
	}

	return nil
}

func mustReadLine(r *bufio.Reader, prompt string) string {
	for {
		s := readLine(r, prompt)
		if strings.TrimSpace(s) != "" {
			return s
		}
		fmt.Println("  value is required, please try again.")
	}
}

func readLine(r *bufio.Reader, prompt string) string {
	fmt.Print(prompt)
	text, _ := r.ReadString('\n')
	return strings.TrimSpace(text)
}

func mustReadYesNo(r *bufio.Reader, prompt string) bool {
	fmt.Print(prompt)
	for {
		s, _ := r.ReadString('\n')
		s = strings.TrimSpace(strings.ToLower(s))
		if s == "" || s == "n" || s == "no" {
			return false
		}
		if s == "y" || s == "yes" {
			return true
		}
		fmt.Print("  please answer y or n: ")
	}
}

func mustReadPassword(prompt string) string {
	fmt.Print(prompt)
	// ใช้ x/term เพื่อซ่อนการพิมพ์ secret
	pwBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println() // ขึ้นบรรทัดใหม่หลังพิมพ์รหัสผ่าน
	if err != nil {
		fmt.Println("  (warn) cannot hide input, fall back to visible")
		reader := bufio.NewReader(os.Stdin)
		return mustReadLine(reader, "")
	}
	pw := strings.TrimSpace(string(pwBytes))
	for pw == "" {
		reader := bufio.NewReader(os.Stdin)
		pw = mustReadLine(reader, "  Secret cannot be empty. Enter again: ")
	}
	return pw
}

func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".mycli")
}

func configPath() string {
	return filepath.Join(configDir(), "config.json")
}

func saveConfigFile(cfg Config) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.Create(configPath())
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(cfg)
}

func loadConfig() (Config, error) {
	var cfg Config
	f, err := os.Open(configPath())
	if err != nil {
		return cfg, err
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}
