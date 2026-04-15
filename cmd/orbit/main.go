package main

import (
	"os"

	"github.com/nakagami-306/orbit/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
