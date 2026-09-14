package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("agentlens %s\n", version)
		return
	}

	fmt.Fprintln(os.Stderr, "agentlens is under early development (v0.1).")
	fmt.Fprintln(os.Stderr, "Available commands: version")
	os.Exit(1)
}
