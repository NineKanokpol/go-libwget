// /Users/nineimacm2/Documents/test-go-wget/main.go
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	// mycli -version
	if *showVersion {
		fmt.Println(version)
		return
	}

	// ถ้าไม่มีคำสั่งตามหลัง -> โชว์ข้อความเดิม
	args := flag.Args()
	if len(args) == 0 {
		fmt.Println("Hello world this is My Lib")
		return
	}

	// mycli pwd         -> รัน /usr/bin/pwd (ตาม PATH)
	// mycli ls -la      -> รัน ls -la
	cmdName := args[0]
	cmdArgs := args[1:]

	cmd := exec.Command(cmdName, cmdArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// ส่งต่อ exit code ที่แท้จริง (ถ้ามี) เพื่อให้สคริปต์เช็คสถานะได้
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintln(os.Stderr, "run error:", err)
		os.Exit(1)
	}
}
