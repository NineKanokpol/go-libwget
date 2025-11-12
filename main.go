package main

import (
	"flag"
	"fmt"
)

var version = "dev" // can be overridden by -ldflags

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	fmt.Println("Hello world this is My LibTTTT")
}
