package main

import (
	"fmt"
	"os"

	"github.com/armandmcqueen/ccsessions/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "ccsessions: "+err.Error())
		os.Exit(1)
	}
}
