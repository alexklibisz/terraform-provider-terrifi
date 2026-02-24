package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "terrifi",
		Short: "Terrifi CLI — tools for managing UniFi infrastructure with Terraform",
	}

	rootCmd.AddCommand(generateImportsCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
