package cli

import (
	"os"

	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

// Run executes root, writes any error to stderr in the structured contract
// (output.WriteError), and exits: 0 on success, 1 on failure. It is the one-line
// main:
//
//	func main() { cli.Run(newRoot()) }
func Run(root *cobra.Command) {
	if err := root.Execute(); err != nil {
		output.WriteError(os.Stderr, err)
		os.Exit(1)
	}
}
