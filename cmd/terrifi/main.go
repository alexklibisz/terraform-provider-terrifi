package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is set at build time via ldflags:
//
//	go build -ldflags "-X main.version=v1.0.0"
//
// GoReleaser sets this automatically.
var version = "dev"

func main() {
	rootCmd := &cobra.Command{
		Use:     "terrifi",
		Short:   "Terrifi CLI — tools for managing UniFi infrastructure with Terraform",
		Version: version,
	}

	rootCmd.AddCommand(generateImportsCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
