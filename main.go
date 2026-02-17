package main

import (
	"context"
	"flag"
	"log"

	"github.com/alexklibisz/terrifi/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

// Terraform providers are standalone binaries that Terraform launches as child processes.
// They communicate with Terraform over gRPC using a protocol defined by HashiCorp.
//
// The provider binary's name matters: it must be "terraform-provider-<name>" for Terraform
// to discover it. Our Taskfile builds this as "terraform-provider-terrifi" explicitly.
// The providerserver package handles all the gRPC plumbing — we just give it our
// provider implementation and it does the rest.
func main() {
	// The debug flag lets you attach a debugger (like delve) to the running provider.
	// Without it, Terraform launches the provider as a subprocess which is hard to debug.
	// With --debug, the provider prints connection info to stdout and waits for Terraform
	// to connect, giving you time to attach a debugger.
	var debug bool

	flag.BoolVar(
		&debug,
		"debug",
		false,
		"set to true to run the provider with support for debuggers like delve",
	)
	flag.Parse()

	opts := providerserver.ServeOpts{
		// Address is the provider's registry address. This tells Terraform how to map
		// the "alexklibisz/terrifi" in a required_providers block to this binary.
		// Format: <hostname>/<namespace>/<type>
		// Even during local development with dev_overrides, this must match what's
		// in the Terraform config's required_providers block.
		Address: "registry.terraform.io/alexklibisz/terrifi",
		Debug:   debug,
	}

	// Serve starts the gRPC server and blocks until Terraform disconnects.
	// provider.New is a factory function — the framework calls it to create a fresh
	// provider instance for each Terraform operation (plan, apply, etc.).
	err := providerserver.Serve(context.Background(), provider.New, opts)
	if err != nil {
		log.Fatal(err.Error())
	}
}
