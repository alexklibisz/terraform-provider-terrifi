package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// version is set at build time via ldflags:
//
//	go build -ldflags "-X main.version=v1.0.0"
//
// GoReleaser sets this automatically. For `go install ...@<version>` builds,
// the version is read from the embedded Go module build info as a fallback.
var version = "dev"

func init() {
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}
	}
}

func main() {
	rootCmd := &cobra.Command{
		Use:     "terrifi",
		Short:   "Terrifi CLI — tools for managing UniFi infrastructure with Terraform",
		Version: version,
	}

	rootCmd.AddCommand(generateImportsCmd())
	rootCmd.AddCommand(checkConnectionCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
