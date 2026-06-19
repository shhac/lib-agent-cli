# AGENTS.md — lib-agent-cli

Guidance for an agent (or human) working in this repo. `CLAUDE.md` is a symlink
to this file.

## What this is

`lib-agent-cli` (module `github.com/shhac/lib-agent-cli`) is the shared CLI
runtime for the agent-first tool family: the copied cobra scaffolding and
credential plumbing. It sits *above* `lib-agent-output` (the zero-dep wire
contract) and is allowed to have dependencies (cobra, `os/exec`, eventually a
dialog lib).

Read [`design-docs/design.md`](design-docs/design.md) for the rationale, the
survey it came from, and the shared-vs-domain boundary per piece.

## Layout

| Path | Responsibility |
|---|---|
| `creds/` | `ConfigDir`, `Store` (0600 JSON), `Keychain` (macOS `security`), `FirstNonEmpty`/`Getenv` |
| `cli/` | `NewRoot`, `Options`/`Globals`, `HandleUnknownCommand`, `Run` |
| `dialog/` | `PromptSecret`/`Prompt`/`Available` — the `--form` native secret dialog (zenity) |
| `examples/demo/` | a complete tiny CLI (incl. `login --form`); the e2e smoke test |

## Build, test, verify

```sh
go build ./...
go vet ./...
go test ./...
```

Always run `go vet` and `go test ./...` before considering a change done.

## The load-bearing principle: mechanism here, policy/domain in the CLI

This module owns the *mechanism* (where files live, how secrets are stored, the
cobra wiring). The CLI supplies the *domain inputs*: the app name, keychain
service, env-var names, credential schema, domain flags, and a config-defaults
hook. Keep that line clean — if a piece encodes knowledge about a specific API
(token formats, which fields are secret, parse-curl, browser cookie extraction,
profile/workspace models, retry/backoff, truncation), it does **not** belong
here.

## Conventions

- **Code style**: early returns; self-documenting code; comments explain *why*.
- **Security-sensitive surface.** `creds` is the one audited place for `0600`
  permissions and keychain handling — don't loosen them, and don't add a path
  that writes secrets to a world-readable file or to argv. `dialog` exists so a
  token never has to transit argv at all (`--form` → native prompt); keep that
  invariant, and keep `Available()` returning a structured error (not a panic)
  on headless hosts.
- **Keychain is injectable.** `Keychain.run` is overridable so tests never touch
  the real keychain; keep it that way. `Available()` gates on `runtime.GOOS`.
- **Depends on lib-agent-output** (published tag in `go.mod`). The error/format
  contract comes from there; don't re-implement `Error`/`FixableBy`/`Format`.

## Naming convention (family-wide)

- `lib-agent-*` = shared libraries (`lib-agent-output`, this repo).
- `agent-*` = the CLIs that consume them.

## Design docs

`design-docs/` holds durable rationale. Record boundary decisions there (why a
piece is shared vs domain) so future changes don't relitigate them.
