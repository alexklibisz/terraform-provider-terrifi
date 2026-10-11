//go:build !acc

package provider

import (
	"fmt"
	"os"
	"testing"
)

// runDockerTests is a stub for builds without the `acc` tag. The real
// implementation lives in provider_docker_acc_test.go and is the only thing in
// this module that imports testcontainers-go/modules/compose.
//
// TestMain stays untagged so unit tests keep running normally; only the Docker
// bootstrap is tag-gated.
func runDockerTests(_ *testing.M) int {
	fmt.Fprintln(os.Stderr,
		"TERRIFI_ACC_TARGET=docker requires the `acc` build tag: go test -tags=acc ./... "+
			"(or use the Taskfile's test:acc targets). Use TERRIFI_ACC_TARGET=hardware "+
			"or TERRIFI_ACC_TARGET=uos to run against an existing controller instead.")
	return 1
}
