# Migrating a CLI onto `lib-agent-cli`

How to replace a hand-rolled CLI **runtime** — credential storage, config-dir
logic, the cobra root scaffolding, and the `--form` secret dialog — with
`github.com/shhac/lib-agent-cli`. This guide is generic; it applies to any
`agent-*` family CLI.

It is the companion to
[`lib-agent-output`'s `MIGRATION_HELP.md`](https://github.com/shhac/lib-agent-output/blob/main/MIGRATION_HELP.md),
which covers the **wire contract** (NDJSON, `Error`/`FixableBy`, `Format`,
pagination). A full migration does both: `lib-agent-output` first (it has no
dependencies and `lib-agent-cli` builds on it), then this. Do the output swap,
get green, then come here.

## What you get vs. what stays yours

`lib-agent-cli` owns the **mechanism**; your CLI keeps the **domain inputs**.

**Owned by `lib-agent-cli` (delete your copies, import these):**

| Your code | `lib-agent-cli` |
|---|---|
| `configBase()` / `ConfigDir()` XDG logic | `xdg.ConfigDir(app)` (also `Cache`/`Data`/`State`/`RuntimeDir`) |
| credential/config file load+save (`0600` JSON) | `creds.Store{Path}.Load/Save` |
| macOS keychain `security` wrapper | `creds.NewKeychain(service)` (`Get`/`Set`/`Delete`/`Available`) |
| `firstNonEmpty(...)` resolution helper | `creds.FirstNonEmpty(...)`, `creds.Getenv(names...)` |
| `newRootCmd` boilerplate (SilenceUsage/Errors, `--format`/`--timeout`/`--debug`, `--format` validation) | `cli.NewRoot(cli.Options{...})` + `cli.Globals` |
| `HandleUnknownCommand`-style helper | `cli.HandleUnknownCommand(root, hint)` (built into `NewRoot`) |
| `main.go` `Execute()` + `WriteError` + `os.Exit(1)` | `cli.Run(root)` |
| `PromptSecret` / `--form` native dialog + SSH/headless check | `dialog.PromptSecret` / `dialog.Prompt` / `dialog.Available` |

**Stays in your CLI (domain — `lib-agent-cli` deliberately won't take it):**

- **The credential schema** (your `Credentials`/`Workspace`/`Profile` structs).
- **Env-var names** (`SLACK_TOKEN`, `AGENT_FOO_PROFILE`, …) and the **profile /
  workspace model**.
- **Token formats / validation** (xoxc/xoxp prefixes, opaque tokens).
- **The keychain service name** (`app.paulie.agent-foo`) and the **placeholder
  strategy** (store secret in keychain, write a `"__KEYCHAIN__"` marker to the
  file) — that orchestration is a few lines of policy; `lib-agent-cli` gives you
  the keychain and the store, you decide the layering.
- **parse-curl, browser/desktop cookie import** — genuinely domain.
- **Error classification** (use `output.FixableByStatus` for the HTTP half;
  GraphQL/vendor classification stays yours), **retry/backoff loops**, and
  **field truncation** — see the `lib-agent-output` guide and design docs.

---

## Step 0 — Assess

```sh
go get github.com/shhac/lib-agent-cli@latest      # pulls lib-agent-output too
git grep -l 'internal/credential\|internal/config\|internal/dialog' | wc -l
```

Map your symbols with the table above. The runtime swap is mostly mechanical;
the judgment calls are the four gotchas below.

---

## Step 1 — `creds`

Replace your config-dir + file store + keychain:

```go
import (
	"github.com/shhac/lib-agent-cli/creds"
	"github.com/shhac/lib-agent-cli/xdg"
)

store := creds.Store{Path: filepath.Join(xdg.ConfigDir("agent-foo"), "credentials.json")}
var c Credentials                 // YOUR schema stays
_ = store.Load(&c)                // missing file → empty, no error
// …mutate c…
_ = store.Save(c)                 // 0600, parent dirs 0700

kc := creds.NewKeychain("app.paulie.agent-foo")   // your service name
if secret, ok := kc.Get(profile); ok { /* … */ }
```

Resolution order (`flag → env → stored default`) keeps its shape; only the env
names are yours:

```go
token := creds.FirstNonEmpty(
    flags.APIKey,
    creds.Getenv("FOO_TOKEN", "AGENT_FOO_TOKEN"),   // vendor name, then agent name
    keychainOrConfigDefault,
)
```

---

## Step 2 — `cli` root

Replace `newRootCmd`:

```go
import "github.com/shhac/lib-agent-cli/cli"

g := &cli.Globals{}   // --format / --timeout / --debug
root := cli.NewRoot(cli.Options{
    Use: "agent-foo", Short: "…", Version: version,
    Globals:        g,
    DefaultFormat:  output.FormatNDJSON,
    ConfigDefaults: func(_ *cobra.Command) { applyPersistedDefaults(g) /* and client setup */ },
    UnknownHint:    "run 'agent-foo usage'",
})
root.PersistentFlags().StringVar(&domainFlag, "scope", "", "…")  // YOUR flags
root.AddCommand(/* subcommands */)
```

and `main.go`:

```go
func main() { cli.Run(newRoot(version)) }   // executes, writes structured error, exits 1
```

---

## Step 3 — `dialog` (the `--form` flag)

Replace your zenity wrapper and availability check:

```go
import "github.com/shhac/lib-agent-cli/dialog"

cmd.Flags().Bool("form", false, "Prompt for the token via a native dialog (never via argv)")
// in RunE:
if form, _ := cmd.Flags().GetBool("form"); form {
    token, err = dialog.PromptSecret(cmd.Context(), "agent-foo: "+profile, "API token")
    if err != nil {
        // dialog returns a NEUTRAL error; map it to your envelope:
        cat, hint := dialog.ClassifyError(err) // CategoryHuman | CategoryRetry | CategoryAgent
        return output.New(err.Error(), output.FixableBy(cat)).WithHint(hint)
    }
}
```

Two-secret flows (e.g. a token **and** a cookie) use the multi-field form
(`InputType: dialog.Password` masks the entry; `dialog.Text` is plain;
`Initial` pre-fills a stored value being edited):

```go
res, err := dialog.Prompt(ctx, dialog.Spec{Title: "agent-foo", Items: []dialog.Field{
    {ID: "token", Label: "API token", InputType: dialog.Password},
    {ID: "cookie", Label: "Session cookie", InputType: dialog.Password},
}})
```

---

## The gotchas (where the judgment is)

### 1. Keychain placeholder strategy stays yours
`lib-agent-cli` gives you the keychain wrapper and the file store as separate
pieces. The pattern of "store the secret in the keychain, write a
`"__KEYCHAIN__"` placeholder to the file, and on read swap the placeholder for
the keychain value" is a few lines of orchestration that stay in your CLI — it's
policy (keychain-managed vs plaintext fallback), and it depends on your schema.

### 2. File permissions: `Store` is always `0600`
If your CLI wrote credentials `0644` *because the secret lives in the keychain*
(only a placeholder is on disk, e.g. `lin`), moving to `0600` is harmless — just
stricter. If you ever wrote a real secret `0644`, this **fixes a leak**; confirm
the new file is `0600` after migrating.

### 3. Pre-run setup goes in `ConfigDefaults`
`NewRoot` owns `PersistentPreRunE` (it runs `ConfigDefaults`, then validates
`--format`). Put your persisted-config application **and** client configuration
there — neither needs the validated format. If you have pre-run logic that must
run *after* `--format` is validated, that isn't expressed by the current
`Options`; flag it (this is exactly the kind of rough edge the first real
migration is meant to surface and refine — `lib-agent-cli` is pre-1.0).

### 4. `dialog` availability is a structured error, not a crash
On a headless host `dialog.Available()` (called inside `Prompt`) returns a
**neutral** error wrapping `dialog.ErrNoGUI`/`ErrUnsupported` — the dialog
package deliberately does NOT know your `fixable_by` taxonomy. Map it with
`dialog.ClassifyError(err)` → `(Category, hint)` and fold `Category` into your
own envelope (`CategoryHuman`/`CategoryRetry`/`CategoryAgent` line up with
`fixable_by`). Still offer a non-GUI path (env var or flag) so an agent on a
headless box can authenticate without the dialog.

---

## Step 4 — Verify

1. **Credential round-trip**: `agent-foo auth add … && agent-foo auth list`, then
   `stat -f '%Sp' <credentials.json>` → confirm `-rw-------` (`0600`).
2. **Keychain** (macOS): confirm the secret lands in the login keychain and the
   file holds only the placeholder (if you use that strategy).
3. **`--form`** on a machine with a GUI: confirm the prompt opens and the token
   never appears in shell history / argv. On a headless box, confirm the
   structured error + your fallback.
4. **`go test ./...`** — update tests for the new import paths / `output.Error`.
5. Unknown-command + `--format bogus` both return structured `fixable_by:agent`
   errors (now from `lib-agent-cli`, not your copy).

---

## Step 5 — Cleanup

- Delete `internal/credential/`, `internal/dialog/`, and the path/store logic in
  `internal/config/` (keep the *schema* and any TTL/settings that are domain).
- Reduce `internal/cli/root.go` to: build `Globals`, call `cli.NewRoot`, add
  domain flags + subcommands.
- `go mod tidy` — your direct `zenity` dependency disappears (it now comes
  transitively via `lib-agent-cli/dialog`), as do your copies' deps.
- `git grep 'internal/credential\|internal/dialog\|configBase\|firstNonEmpty'`
  should come back empty (or only your retained domain code).

---

## What this leaves

After both migrations (`lib-agent-output` + `lib-agent-cli`), a typical CLI is
its *domain*: API client, resource mappers, command tree, credential schema,
token formats, and error/redaction specifics — with the entire output contract
and CLI runtime imported, not copied.
