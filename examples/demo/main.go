// Command demo is a tiny CLI built with lib-agent-cli — it wires the root
// builder and a creds-backed store together, and serves as the module's
// end-to-end smoke test.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/shhac/lib-agent-cli/cli"
	"github.com/shhac/lib-agent-cli/creds"
	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

const appName = "lib-agent-cli-demo"

type profile struct {
	DefaultAccount string            `json:"default_account,omitempty"`
	Accounts       map[string]string `json:"accounts,omitempty"`
}

func store() creds.Store {
	return creds.Store{Path: filepath.Join(creds.ConfigDir(appName), "credentials.json")}
}

func main() {
	g := &cli.Globals{}
	root := cli.NewRoot(cli.Options{
		Use:           "demo",
		Short:         "Demo CLI built on lib-agent-cli",
		Version:       "0.1.0",
		Globals:       g,
		DefaultFormat: output.FormatNDJSON,
		UnknownHint:   "run 'demo --help' for usage",
	})

	root.AddCommand(&cobra.Command{
		Use:   "whoami",
		Short: "Show the default account",
		RunE: func(_ *cobra.Command, _ []string) error {
			var p profile
			if err := store().Load(&p); err != nil {
				return output.Wrap(err, output.FixableByHuman)
			}
			if p.DefaultAccount == "" {
				return output.New("no default account set", output.FixableByHuman).
					WithHint("set one with 'demo login <account>'")
			}
			return output.NewNDJSONWriter(os.Stdout).WriteItem(map[string]any{"account": p.DefaultAccount})
		},
	})

	root.AddCommand(&cobra.Command{
		Use:   "login [account]",
		Short: "Store an account as the default",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			var p profile
			if err := store().Load(&p); err != nil {
				return output.Wrap(err, output.FixableByHuman)
			}
			if p.Accounts == nil {
				p.Accounts = map[string]string{}
			}
			p.Accounts[args[0]] = fmt.Sprintf("token-for-%s", args[0])
			p.DefaultAccount = args[0]
			if err := store().Save(p); err != nil {
				return output.Wrap(err, output.FixableByHuman)
			}
			return output.NewNDJSONWriter(os.Stdout).WriteItem(map[string]any{"logged_in": args[0]})
		},
	})

	cli.Run(root)
}
