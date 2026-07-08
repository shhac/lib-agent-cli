# Building an agent-first CLI from scratch

How to build a brand-new `agent-*` CLI on the two shared libraries, starting
from an empty directory. This is the **from-scratch** companion to
[`MIGRATION_HELP.md`](MIGRATION_HELP.md) (which is for retrofitting an *existing*
hand-rolled CLI). If you already have a working CLI, read that one instead.

Everything below uses the **current** API
(`github.com/shhac/lib-agent-cli@v0.4.2`,
`github.com/shhac/lib-agent-output@v0.4.3`) and compiles against it. The
kitchen-sink reference is [`examples/demo/main.go`](examples/demo/main.go) — the
snippets here mirror it; when in doubt, read that file.

---

## 1. What you get / the mental model

Two libraries, one boundary:

- **`lib-agent-output`** is the zero-dependency **wire contract**: the
  `{error, fixable_by, hint}` error shape, the `Format` selector, NDJSON list
  streaming, pagination, pruning, and redaction. Nothing CLI-specific; nothing
  about cobra.
- **`lib-agent-cli`** is the **runtime** on top of it: the cobra root builder,
  the shared `--format`/`--timeout`/`--debug` flags, the XDG config-dir
  resolver, the `0600` credential store, the macOS keychain wrapper, and the
  `--form` native secret dialog.

The split is deliberate: **the libraries own the mechanism; your CLI supplies
the domain inputs** — the app name, the env-var names, the credential schema,
the API client, and the command tree. Nothing in either library knows about any
specific API. You wire the pieces together and add the domain.

---

## 2. Scaffold

```sh
mkdir agent-foo && cd agent-foo
go mod init github.com/you/agent-foo
go get github.com/shhac/lib-agent-cli@latest     # v0.4.2 — pulls lib-agent-output too
go get github.com/shhac/lib-agent-output@latest  # v0.4.3 — pin it explicitly
```

(`go get lib-agent-cli` already brings `lib-agent-output` in transitively, but
pin it directly so you control the wire-contract version.)

Lay the tool out with the binary under `cmd/agent-foo` and the domain code in
packages you own:

```
agent-foo/
├── go.mod
├── cmd/agent-foo/main.go     # the one-line main
└── internal/cli/root.go      # builds the root, adds subcommands
```

`cmd/agent-foo/main.go` is the whole `main`:

```go
package main

import "github.com/you/agent-foo/internal/cli"

func main() { cli.Run() } // cli.Run wraps the lib's cli.Run — see §3
```

---

## 3. The root command

`cli.NewRoot` builds the cobra root with the family conventions baked in:
`SilenceUsage`/`SilenceErrors` on, the shared persistent flags bound,
`--format` validated up front, and a structured unknown-subcommand handler.
You add domain persistent flags and subcommands to the returned `*cobra.Command`.

```go
package cli

import (
	libcli "github.com/shhac/lib-agent-cli/cli"
	"github.com/shhac/lib-agent-cli/creds"
	output "github.com/shhac/lib-agent-output"
	_ "github.com/shhac/lib-agent-cli/yaml" // enables --format yaml (see §5)
)

const version = "0.1.0"

func newRoot() *cobra.Command {
	g := &libcli.Globals{} // --format / --timeout / --debug live here
	root := libcli.NewRoot(libcli.Options{
		Use:           "agent-foo",
		Short:         "Foo CLI for agents",
		Version:       version,
		Globals:       g,
		DefaultFormat: output.FormatNDJSON, // lists stream NDJSON by default
		ConfigDefaults: func(cmd *cobra.Command) {
			// Runs in PersistentPreRunE *before* --format is validated. Apply
			// persisted defaults and set up your API client here. cmd lets you
			// scope a persisted default to a command class (cli.FormatAllowed).
			g.TimeoutMS = creds.FirstNonZero(g.TimeoutMS, 30000)
		},
		UnknownHint: "run 'agent-foo usage' for the command overview",
	})

	// Domain persistent flags go on the returned root:
	root.PersistentFlags().StringVar(&apiKeyFlag, "api-key", "", "API key (overrides env/config)")

	root.AddCommand(
		libcli.ConfigCommand(g, configKeys()), // §6
		usageCmd(),                         // §11
		// …your domain subcommands…
	)
	return root
}

func Run() { libcli.Run(newRoot()) }
```

The embedded `Globals` struct is just the three shared flags:

```go
type Globals struct {
	Format    string
	TimeoutMS int
	Debug     bool
}
```

You read them inside `RunE` (e.g. `g.TimeoutMS` for the request deadline,
`g.Format` to pick the output format). Keep a pointer to `g` in scope —
capture it in a closure or stash it on a struct your commands can reach.

**`ConfigDefaults` is the only pre-run hook.** It receives the command being
executed and runs before `--format` is validated, so it's the place for applying persisted config and constructing
your API client. If you have setup that must run *after* `--format` is parsed,
the current `Options` can't express it — do it lazily inside `RunE` instead.

---

## 4. Output: the wire contract

Two output shapes cover almost everything, and **every error reaches stderr as
structured JSON**.

### Lists → NDJSON (the default)

A list streams one record per line. Use the `NDJSONWriter` directly when you're
generating records as you go:

```go
func RunE(cmd *cobra.Command, _ []string) error {
	w := output.NewNDJSONWriter(os.Stdout)
	for _, item := range items {
		if err := w.WriteItem(map[string]any{"id": item.ID, "name": item.Name}); err != nil {
			return err
		}
	}
	// trailing pagination line, only when there's more:
	return w.WritePagination(output.Pagination{HasMore: true, NextCursor: cursor})
}
```

…or `output.WriteList` when you have the full slice up front and want the format
to drive the shape (NDJSON streams; JSON/YAML collect into a `{"data":[…]}`
envelope):

```go
func RunE(cmd *cobra.Command, _ []string) error {
	format, err := output.ResolveFormat(g.Format, output.FormatNDJSON)
	if err != nil {
		return output.Wrap(err, output.FixableByAgent)
	}
	items := []any{ /* records */ }
	meta := map[string]any{output.MetaKeyPagination: output.Pagination{HasMore: false}}
	return output.WriteList(os.Stdout, format, items, meta, output.PruneEmpty)
}
```

### Single resources → `Print`

For one resource, `output.Print` honours the format (`FormatJSON` is the
single-resource default — pretty-printed):

```go
format, _ := output.ResolveFormat(g.Format, output.FormatJSON)
return output.Print(os.Stdout, resource, format, output.PruneEmpty)
```

Pass a `Pruner` (`output.PruneNils`, `output.PruneEmpty`, or your own) to strip
empty fields, or `nil` to write the value verbatim.

### Errors → `{error, fixable_by, hint}`

Every failure path returns an `*output.Error`. `cli.Run` (via
`output.WriteError`) writes it as a single JSON line to stderr and exits 1:

```go
return output.New("workspace not found: "+name, output.FixableByAgent).
	WithHint("run 'agent-foo workspace list' to see valid names")
```

Construction helpers: `output.New(msg, fixableBy)`, `output.Newf(fixableBy,
format, args…)`, `output.Wrap(err, fixableBy)` (preserves the cause; a `nil`
error wraps to `nil` so you can wrap-and-return without a guard). Chain
`.WithHint(…)`, `.WithHints(…)`, or `.WithRetryAfter(d)` (sets
`retry_after_seconds`).

**Choosing `fixable_by`** — the taxonomy tells the calling agent what to do:

| value                    | meaning | agent's next move |
|--------------------------|---------|-------------------|
| `output.FixableByAgent`  | bad input (args, flags, target) — 4xx-class | fix its own input and retry |
| `output.FixableByHuman`  | auth, permission, payment, or an explicit confirmation it must not self-grant | stop and defer to a person |
| `output.FixableByRetry`  | transient (429/5xx/network/timeout) | back off and retry the same call |

For HTTP responses, let `output.FixableByStatus(code)` pick (401/402/403 →
human; 429/5xx → retry; everything else → agent), then override per-vendor if
needed.

### The silent-usage trap

`NewRoot` sets `SilenceUsage` and `SilenceErrors` so cobra never prints its own
usage text or error to stderr — otherwise a bad flag would emit unstructured
text that breaks an agent parsing your output. Because of that, **every error
must come back as an `*output.Error`** (or a plain `error`, which
`output.WriteError` classifies as `fixable_by: agent`). Two places matter:

- **Unknown subcommands** are already handled — `NewRoot` installs
  `HandleUnknownCommand`, which returns a `fixable_by: agent` error listing the
  valid commands plus your `UnknownHint`.
- **A bad `--format` value** is validated in `PersistentPreRunE` and comes back
  as a `fixable_by: agent` error.

If you add your own flag validation, do it in `RunE` and return an
`*output.Error` — don't let cobra's argument parser print to stderr unstructured.

---

## 5. `--format yaml`

`lib-agent-output` is dependency-free and so handles only JSON and NDJSON
natively. YAML needs an encoder, which lives in `lib-agent-cli`'s `yaml`
package. Enable `--format yaml` with a blank import — its `init` registers the
encoder:

```go
import _ "github.com/shhac/lib-agent-cli/yaml"
```

After that, `output.Print(…, output.FormatYAML, …)` and `output.WriteList(…,
output.FormatYAML, …)` emit YAML. Note the asymmetry: **NDJSON streams**
record-by-record, while **YAML and JSON collect** into a `{"data":[…]}`
envelope. The `--format` flag already advertises `json, yaml, jsonl`.

---

## 6. Config

Persisted settings live under the XDG config dir, in a `0600` JSON file managed
by `creds.Store`.

```go
import (
	"path/filepath"
	"github.com/shhac/lib-agent-cli/creds"
	"github.com/shhac/lib-agent-cli/xdg"
)

const appName = "agent-foo"

type settings struct {
	DefaultWorkspace string `json:"default_workspace,omitempty"`
	PageSize         int    `json:"page_size,omitempty"`
}

func settingsStore() creds.Store {
	return creds.Store{Path: filepath.Join(xdg.ConfigDir(appName), "config.json")}
}

func loadSettings() settings {
	var s settings
	_ = settingsStore().Load(&s) // missing file → empty, no error
	return s
}

func saveSettings(s settings) error { return settingsStore().Save(s) }
```

`xdg.ConfigDir(app)` resolves `$XDG_CONFIG_HOME/<app>` (else `~/.config/<app>`);
there's a matching `CacheDir`/`DataDir`/`StateDir`/`RuntimeDir`, and an
`xdg.App{Name: appName}` bundle if you'd rather hold one value. `Store.Save`
writes `0600` with parent dirs `0700`; `Store.Load` treats a missing file as
empty.

Expose get/set/unset/list with `cli.ConfigCommand` — you supply the `Globals`
(so `--format` is honored) and typed closures per key, the lib owns the cobra
scaffolding and output. Every verb emits the key's `{key, value, set}` state
(NDJSON by default; `--format json|yaml` gives the bare object, or a
`{"data":[…]}` envelope for `list`):

```go
func configKeys() []libcli.ConfigKey {
	return []libcli.ConfigKey{
		{
			Name:        "default_workspace",
			Description: "Workspace used when none is given",
			Get:   func() (string, bool) { s := loadSettings(); return s.DefaultWorkspace, s.DefaultWorkspace != "" },
			Set:   func(v string) error { s := loadSettings(); s.DefaultWorkspace = v; return saveSettings(s) },
			Unset: func() error { s := loadSettings(); s.DefaultWorkspace = ""; return saveSettings(s) },
		},
	}
}
```

A `nil` `Set` marks a key read-only; a `nil` `Unset` makes it un-clearable.
Unknown keys produce a `fixable_by: agent` error listing the valid ones.

---

## 7. Credentials & secrets

The secret itself goes in the macOS keychain (when available); the config file
holds only non-secret settings or a placeholder. `creds.NewKeychain` wraps the
`security` CLI:

```go
kc := creds.NewKeychain("app.paulie.agent-foo") // family reverse-domain convention
if kc.Available() {                              // macOS only
	_ = kc.Set(workspace, token)                 // add-generic-password
}
secret, ok := kc.Get(workspace)                  // find-generic-password
```

`Keychain` offers `Get`/`Set`/`Delete`/`DeleteAll`/`Available`. The service name
is yours (the **`app.paulie.<tool>`** family convention) — the CLI owns the
prefix; the library is service-agnostic.

**Keychain-managed vs file-placeholder.** The library hands you the keychain and
the store as separate pieces; the *policy* of "store the secret in the keychain,
write a `"__KEYCHAIN__"` marker to the file, swap it back on read" is a few lines
that stay in your CLI (it depends on your schema). On a host without a keychain,
fall back to writing the secret into the `0600` store directly.

**Resolution precedence** is `flag → env → stored default`, built from
`creds.FirstNonEmpty` and `creds.Getenv`:

```go
token := creds.FirstNonEmpty(
	apiKeyFlag,                                  // --api-key
	creds.Getenv("FOO_TOKEN", "AGENT_FOO_TOKEN"), // vendor name, then agent name
	keychainOrConfigDefault,                      // resolved last
)
```

`creds.FirstNonZero` is the int analog (timeouts, page sizes) where `0` means
"unset". Note the security property of NDJSON + env/keychain resolution: **the
secret is never passed on argv**, so it can't leak into shell history or a
process listing.

---

## 8. `--form` secret entry

When a token isn't in the environment or keychain, prompt for it via a native OS
dialog so it still never transits argv. `dialog.PromptSecret` is the
single-field convenience:

```go
import "github.com/shhac/lib-agent-cli/dialog"

cmd.Flags().Bool("form", false, "Prompt for the token via a native dialog (never via argv)")
// in RunE:
if form, _ := cmd.Flags().GetBool("form"); form {
	token, err := dialog.PromptSecret(cmd.Context(), "agent-foo: "+workspace, "API token")
	if err != nil {
		// dialog returns a NEUTRAL error; map its Category onto your envelope:
		cat, hint := dialog.ClassifyError(err)
		return output.Wrap(err, output.FixableBy(string(cat))).WithHint(hint)
	}
	// …store token…
}
```

For a multi-secret flow (e.g. a token **and** a cookie) use `dialog.Prompt` with
a `Spec`. `InputType: dialog.Password` masks the field, `dialog.Text` is plain,
and `Field.Initial` pre-fills a value being edited:

```go
res, err := dialog.Prompt(cmd.Context(), dialog.Spec{
	Title: "agent-foo",
	Items: []dialog.Field{
		{ID: "token", Label: "API token", InputType: dialog.Password},
		{ID: "cookie", Label: "Session cookie", InputType: dialog.Password},
	},
})
// res is []dialog.Result{{ID, Value}}, in Spec.Items order.
```

**Mapping the neutral `Category` onto `fixable_by`.** The `dialog` package
deliberately doesn't import `lib-agent-output`, so it returns a neutral
`Category` instead of a `fixable_by`. `dialog.ClassifyError(err)` →
`(Category, hint)`: `CategoryHuman` (no GUI / not installed — don't retry),
`CategoryRetry` (user cancelled — re-run), `CategoryAgent` (anything else). The
values line up with `fixable_by`, so `output.FixableBy(string(cat))` is the
whole mapping.

**The headless fallback.** On a headless host `dialog.Available()` (called
inside `Prompt`) returns a neutral error wrapping `dialog.ErrNoGUI` /
`dialog.ErrUnsupported`. Always keep a non-GUI path (the env var or the
`--api-key` flag) so an agent on a headless box can still authenticate — the
dialog is a convenience, not the only door.

**Testing it** — swap the default Prompter for `dialogtest.Recorder` and inspect
what your CLI sent:

```go
import (
	"github.com/shhac/lib-agent-cli/dialog"
	"github.com/shhac/lib-agent-cli/dialog/dialogtest"
)

rec := &dialogtest.Recorder{PromptResults: []dialog.Result{{ID: "secret", Value: "tok"}}}
restore := dialog.SetDefault(rec)
defer restore()
// …run the command…
// rec.Calls holds every Spec passed to Prompt, in order.
// Set rec.AvailableErr / rec.PromptErr to exercise the headless / cancel paths.
```

---

## 9. Destructive operations

Gate any state-changing action behind `--yes`. `cli.AddConfirmFlag` registers
the flag; `cli.RequireConfirm` returns a `fixable_by: human` error (with a
"rerun with --yes" hint) until it's set:

```go
func deleteCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:  "delete <id>",
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := libcli.RequireConfirm(yes, "this permanently deletes "+args[0]); err != nil {
				return err
			}
			// …perform the delete…
			return output.NewNDJSONWriter(os.Stdout).WriteItem(map[string]any{"deleted": args[0]})
		},
	}
	libcli.AddConfirmFlag(cmd, &yes)
	return cmd
}
```

The error is `fixable_by: human` on purpose — an agent must not self-grant a
destructive confirmation.

---

## 10. Conventions checklist

A new `agent-*` CLI follows the family conventions:

- **Structured errors, always.** Every failure path returns an `*output.Error`
  (or a plain error → classified `agent`). Never let cobra print unstructured
  usage/errors — `NewRoot` already silences it.
- **NDJSON list default.** Lists stream NDJSON (`DefaultFormat:
  output.FormatNDJSON`); single resources default to pretty JSON.
- **`--format json|yaml|jsonl`.** Blank-import the `yaml` package to enable YAML.
- **`app.paulie.<tool>` keychain service.** Reverse-domain, your tool name as
  the suffix.
- **Secret never on argv.** Resolve via env/keychain/`--form`; argv is for
  non-secret inputs only.
- **`--yes` for destructive ops**, returning `fixable_by: human` until set.
- **Resolution order** `flag → env → stored default` via
  `creds.FirstNonEmpty` / `creds.Getenv`.
- **A `usage` command** (next section) and a structured unknown-command handler
  (free from `NewRoot`).
- **`fixable_by` chosen deliberately** — agent / human / retry, per §4.

Verify with the standard loop:

```sh
go build ./...
go vet ./...
go test ./...
```

---

## 11. A `usage` command

The family convention is an LLM-oriented `usage` command: a single overview of
what the CLI does, its commands, auth model, and output shape, written for an
agent reading it cold. It's where you point `UnknownHint`. Implement it as a
plain subcommand that prints a structured overview:

```go
func usageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "usage",
		Short: "Overview of agent-foo for an LLM driving it",
		RunE: func(_ *cobra.Command, _ []string) error {
			return output.Print(os.Stdout, map[string]any{
				"tool":       "agent-foo",
				"summary":    "Read and manage Foo resources.",
				"auth":       "FOO_TOKEN / AGENT_FOO_TOKEN env, --api-key flag, or 'agent-foo login --form'.",
				"output":     "NDJSON by default; --format json|yaml|jsonl.",
				"errors":     "stderr JSON: {error, fixable_by: agent|human|retry, hint}.",
				"commands":   []string{"workspace list", "item list", "item delete", "config", "usage"},
			}, output.FormatJSON, nil)
		},
	}
}
```

---

## Further reading

- [`MIGRATION_HELP.md`](MIGRATION_HELP.md) — migrating an *existing* CLI onto
  these libraries (the inverse of this guide).
- [`design-docs/design.md`](design-docs/design.md) — the shared-vs-domain
  boundary, the survey it came from, and why each piece is where it is.
- [`examples/demo/main.go`](examples/demo/main.go) — the kitchen-sink CLI that
  wires every package; the snippets here mirror it.
