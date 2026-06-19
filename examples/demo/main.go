// Command demo is the kitchen-sink CLI for lib-agent-cli: it wires every piece —
// the root builder + shared flags + config-defaults hook, a config command, the
// --yes confirm gate, the --form secret dialog, and the creds store /
// config+cache dirs / resolution helpers — into one small program that also
// serves as the end-to-end smoke test.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/shhac/lib-agent-cli/cli"
	"github.com/shhac/lib-agent-cli/creds"
	"github.com/shhac/lib-agent-cli/dialog"
	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

const appName = "lib-agent-cli-demo"

type settings struct {
	DefaultAccount string `json:"default_account,omitempty"`
	PageSize       int    `json:"page_size,omitempty"`
}

func settingsStore() creds.Store {
	return creds.Store{Path: filepath.Join(creds.ConfigDir(appName), "config.json")}
}

func loadSettings() settings {
	var s settings
	_ = settingsStore().Load(&s)
	return s
}

func saveSettings(s settings) error { return settingsStore().Save(s) }

func main() {
	g := &cli.Globals{}
	root := cli.NewRoot(cli.Options{
		Use:           "demo",
		Short:         "Kitchen-sink CLI exercising lib-agent-cli",
		Version:       "0.1.0",
		Globals:       g,
		DefaultFormat: output.FormatNDJSON,
		ConfigDefaults: func() {
			// Resolve the effective timeout: flag > (a persisted setting, omitted
			// here) > built-in default — demonstrating FirstNonZero.
			g.TimeoutMS = creds.FirstNonZero(g.TimeoutMS, 30000)
		},
		UnknownHint: "run 'demo --help' for usage",
	})

	root.AddCommand(
		cli.ConfigCommand(configKeys()),
		whoamiCmd(),
		loginCmd(),
		logoutCmd(),
		itemListCmd(),
		dirsCmd(),
	)

	cli.Run(root)
}

func configKeys() []cli.ConfigKey {
	return []cli.ConfigKey{
		{
			Name:        "default_account",
			Description: "Account used when none is given",
			Get:         func() (string, bool) { s := loadSettings(); return s.DefaultAccount, s.DefaultAccount != "" },
			Set:         func(v string) error { s := loadSettings(); s.DefaultAccount = v; return saveSettings(s) },
			Unset:       func() error { s := loadSettings(); s.DefaultAccount = ""; return saveSettings(s) },
		},
		{
			Name:        "page_size",
			Description: "Default items per page (1-100)",
			Get: func() (string, bool) {
				s := loadSettings()
				if s.PageSize == 0 {
					return "", false
				}
				return strconv.Itoa(s.PageSize), true
			},
			Set: func(v string) error {
				n, err := strconv.Atoi(v)
				if err != nil || n < 1 || n > 100 {
					return fmt.Errorf("page_size must be an integer in 1..100")
				}
				s := loadSettings()
				s.PageSize = n
				return saveSettings(s)
			},
			Unset: func() error { s := loadSettings(); s.PageSize = 0; return saveSettings(s) },
		},
	}
}

func whoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show the resolved account",
		RunE: func(_ *cobra.Command, _ []string) error {
			// Resolution order: env DEMO_ACCOUNT > stored default.
			acct := creds.FirstNonEmpty(creds.Getenv("DEMO_ACCOUNT"), loadSettings().DefaultAccount)
			if acct == "" {
				return output.New("no account configured", output.FixableByHuman).
					WithHint("run 'demo login <account>' or 'demo config set default_account <name>'")
			}
			return output.NewNDJSONWriter(os.Stdout).WriteItem(map[string]any{"account": acct})
		},
	}
}

func loginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login [account]",
		Short: "Store an account (token in the keychain) as the default; --form prompts for the token",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			token := fmt.Sprintf("token-for-%s", args[0])
			if form, _ := cmd.Flags().GetBool("form"); form {
				t, err := dialog.PromptSecret(cmd.Context(), "demo: "+args[0], "API token")
				if err != nil {
					return err
				}
				token = t
			}
			// Prefer the keychain for the secret (no-op on non-macOS).
			if kc := creds.NewKeychain("app.paulie." + appName); kc.Available() {
				_ = kc.Set(args[0], token)
			}
			s := loadSettings()
			s.DefaultAccount = args[0]
			if err := saveSettings(s); err != nil {
				return output.Wrap(err, output.FixableByHuman)
			}
			return output.NewNDJSONWriter(os.Stdout).WriteItem(map[string]any{"logged_in": args[0]})
		},
	}
	cmd.Flags().Bool("form", false, "Prompt for the token via a native dialog (never via argv)")
	return cmd
}

func logoutCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Clear the stored default account",
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := cli.RequireConfirm(yes, "this clears the stored default account"); err != nil {
				return err
			}
			s := loadSettings()
			s.DefaultAccount = ""
			if err := saveSettings(s); err != nil {
				return output.Wrap(err, output.FixableByHuman)
			}
			return output.NewNDJSONWriter(os.Stdout).WriteItem(map[string]any{"logged_out": true})
		},
	}
	cli.AddConfirmFlag(cmd, &yes)
	return cmd
}

func itemListCmd() *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "item-list",
		Short: "List demo items (page size: --limit > config page_size > 25)",
		RunE: func(_ *cobra.Command, _ []string) error {
			pageSize := creds.FirstNonZero(limit, loadSettings().PageSize, 25)
			const total = 8
			n := pageSize
			if n > total {
				n = total
			}
			w := output.NewNDJSONWriter(os.Stdout)
			for i := 0; i < n; i++ {
				if err := w.WriteItem(map[string]any{"id": fmt.Sprintf("item-%d", i+1)}); err != nil {
					return err
				}
			}
			if n < total {
				return w.WritePagination(output.Pagination{HasMore: true, TotalItems: total})
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Max items (0 = use config page_size, else 25)")
	return cmd
}

func dirsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dirs",
		Short: "Show the config and cache directories",
		RunE: func(_ *cobra.Command, _ []string) error {
			return output.NewNDJSONWriter(os.Stdout).WriteItem(map[string]any{
				"config_dir": creds.ConfigDir(appName),
				"cache_dir":  creds.CacheDir(appName),
			})
		},
	}
}
