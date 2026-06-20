# lib-agent-cli

The shared **CLI runtime** for agent-first command-line tools: the copied
cobra scaffolding and credential plumbing that every `agent-*` tool reimplements
by hand.

Where [`lib-agent-output`](https://github.com/shhac/lib-agent-output) is the
zero-dependency **wire contract** (NDJSON, errors, pagination), `lib-agent-cli`
is the **runtime around it** — the root command, the persistent flags, the
config/credential storage. It depends on `lib-agent-output`, cobra, and the
standard library.

A survey of the family showed this layer is even more copied than the output
layer: the XDG config-dir resolver is byte-identical across tools, the macOS
keychain invocation is identical down to the flags, and the cobra root setup is
~95% the same. This module is the single home for it.

## Packages

| Package | What it provides |
|---|---|
| [`xdg`](xdg) | freedesktop base dirs — `ConfigDir`/`CacheDir`/`DataDir`/`StateDir`/`RuntimeDir` with spec env vars + sensible fallbacks when unset (+ an `App{Name}` bundle: one identity → all paths) |
| [`creds`](creds) | secret plumbing — `Store` (0600 JSON load/save), `Keychain` (macOS `security` wrapper), `FirstNonEmpty`/`FirstNonZero`/`Getenv` value resolvers |
| [`cli`](cli) | `NewRoot(Options)` (cobra root with shared flags + `--format` validation), `ConfigCommand(keys)` (`get`/`set`/`unset`/`list`), `RequireConfirm`/`AddConfirmFlag` (the `--yes` gate), `HandleUnknownCommand`, `Run` |
| [`yaml`](yaml) | opt-in `--format yaml` encoder (registers a yaml.v3 encoder for lib-agent-output's FormatYAML; blank-import to enable) — keeps the core output lib dependency-free |
| [`dialog`](dialog) | the `--form` boilerplate: `Prompter` interface + `PromptSecret`/`Prompt`/`Available` (native OS secret dialog via zenity; tokens never touch argv) + neutral `Category`/`ClassifyError` (no host-error coupling) + `dialogtest.Recorder` |

The CLI supplies the **domain inputs** — app name, keychain service, env-var
names, domain flags, credential schema — and this module owns the **mechanism**.
Nothing here knows about any specific API.

## Quick start

```go
package main

import (
    "path/filepath"

    "github.com/shhac/lib-agent-cli/cli"
    "github.com/shhac/lib-agent-cli/creds"
    "github.com/shhac/lib-agent-cli/xdg"
    output "github.com/shhac/lib-agent-output"
)

func main() {
    g := &cli.Globals{}
    root := cli.NewRoot(cli.Options{
        Use: "agent-foo", Short: "Foo CLI for agents", Version: version,
        Globals: g, DefaultFormat: output.FormatNDJSON,
        UnknownHint: "run 'agent-foo --help'",
    })
    // add domain persistent flags + subcommands to root …
    cli.Run(root)
}

// credentials, stored 0600 under the XDG config dir, secret in the keychain:
store := creds.Store{Path: filepath.Join(xdg.ConfigDir("agent-foo"), "credentials.json")}
kc := creds.NewKeychain("app.paulie.agent-foo")
```

See [`examples/demo`](examples/demo) for a complete tiny CLI (also the e2e smoke
test).

**Building a new CLI from scratch?** [`GETTING_STARTED.md`](GETTING_STARTED.md)
is the from-scratch tutorial — empty directory to a working agent-first CLI.

**Migrating an existing CLI?** [`MIGRATION_HELP.md`](MIGRATION_HELP.md) is the
step-by-step guide (companion to `lib-agent-output`'s, which covers the output
contract).

## Scope

`xdg` (filesystem locations), `creds` (secrets), the `cli` root builder, and
`dialog` (the `--form` secret-entry boilerplate) — the settled pieces that are
copied across the family. The [`design-docs/design.md`](design-docs/design.md)
records the shared-vs-domain boundary for every piece and what deliberately
stays in each CLI (parse-curl, browser import, token formats, retry/backoff,
truncation). Redaction is a wire-shape concern and lives in `lib-agent-output`
(`output.Redact`).

## Develop

```sh
go test ./...
go vet ./...
```

Depends on the published `github.com/shhac/lib-agent-output`. See
[`AGENTS.md`](AGENTS.md).

## License

[PolyForm Perimeter License 1.0.0](LICENSE) — © 2026 Paul Somers.
