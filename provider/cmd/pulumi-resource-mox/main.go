// Command pulumi-resource-mox is the Pulumi plugin binary for the mox provider.
//
// The binary name follows the Pulumi convention "pulumi-resource-<name>"; Pulumi
// launches it and speaks the provider gRPC protocol over stdio.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/hilli/pulumi-mox/provider"
)

func main() {
	prov, err := provider.Provider()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to build mox provider: %v\n", err)
		os.Exit(1)
	}
	if err := prov.Run(context.Background(), "mox", provider.Version); err != nil {
		fmt.Fprintf(os.Stderr, "mox provider exited with error: %v\n", err)
		os.Exit(1)
	}
}
