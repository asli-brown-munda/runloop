package main

import (
	"fmt"
	"os"

	"runloop/internal/cli"
)

func main() {
	if err := cli.NewRootCommand("runloop").Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
