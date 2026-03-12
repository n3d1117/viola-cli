package main

import (
	"os"

	"viola/internal/commands"
)

func main() {
	os.Exit(commands.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
