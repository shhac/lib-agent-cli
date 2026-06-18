# lib-agent-cli: design

The shared **CLI runtime** for agent-first tools — the copied cobra scaffolding
and credential plumbing — sitting above the `lib-agent-output` wire contract.

## Why

A survey of the `agent-*` family (vercel, cloudflare, dd, incident, stripe,
posthog, slack, lin, …) found that the runtime layer is *more* copied than the
output layer that became `lib-agent-output`:

- **XDG config-dir resolver** — byte-identical across slack/posthog/cloudflare/
  lin (posthog ≡ cloudflare verbatim).
- **macOS keychain** — identical `security add/find/delete-generic-password -s
  <service> -a <account> -w` everywhere; only the service name differs.
- **Credential file store** — identical `0o600` JSON load/save.
- **Cobra root scaffolding** — ~95% identical: `SilenceUsage`/`SilenceErrors`,
  `--format`/`--timeout`/`--debug`/`--base-url`, `--format` validation in
  `PersistentPreRunE`, an unknown-subcommand handler, and a `main`→exit-1 wrapper.
- **Secret dialog** — near-identical `zenity.Entry` secret prompt in
  slack/posthog (cloudflare has a richer multi-field variant).

This is also the *security-sensitive* boundary — credential storage is exactly
where you want one audited implementation (0600, keychain, secret-never-in-argv)
rather than a copy per CLI.

## The boundary: mechanism here, domain in the CLI

The same discipline that shaped `lib-agent-output`: absorb the copied,
domain-free *mechanism*; leave the *domain inputs* (and divergent policy) to the
CLI.

| Shared (here) | Domain (stays in the CLI) |
|---|---|
| XDG `ConfigDir(app)` | the app name |
| `Store` (0600 JSON load/save) | the credential schema |
| `Keychain` (`security` wrapper) | the service name, the placeholder strategy |
| `FirstNonEmpty`/`Getenv` resolution helpers | which env-var names, the profile/workspace model |
| cobra root builder, shared flags, `--format` validation, unknown-command handler, `Run` | domain flags, the config-defaults hook, subcommands |
| (planned) secret dialog | the field labels |

Explicitly **not** here (domain or divergent): token formats (xoxc/xoxp/opaque),
parse-curl, browser/desktop cookie extraction, GraphQL-vs-HTTP error
classification (use `output.FixableByStatus` for the HTTP half), retry/backoff
loops (they vary and live in the client layer), and field truncation (lin-only,
field-selection is domain).

## Architecture

```
lib-agent-output   (zero-dep wire contract: NDJSON, Error/FixableBy, Format, Pagination)
        ▲
lib-agent-cli      (cobra + creds runtime; MAY have deps)
   ├─ creds : ConfigDir, Store (0600), Keychain, FirstNonEmpty/Getenv
   ├─ cli   : NewRoot(Options)+Globals, HandleUnknownCommand, Run
   └─ dialog: PromptSecret (planned — see below)
```

`lib-agent-cli` depends on the **published** `lib-agent-output` tag, so a tool
that adopts both deletes its `internal/output/`, `internal/errors/`,
`internal/credential/`, `internal/config/` path logic, and most of
`internal/cli/root.go`.

## Scope of v0.1.0 (this cut) and what's deferred

**In:** `creds` and the `cli` root builder — the byte-identical, fully testable,
dependency-light pieces (deps: cobra + lib-agent-output only).

**Deferred — `dialog` (secret entry via native OS prompt):** slack/posthog use a
plain `zenity.Entry`; cloudflare uses a richer multi-field `Prompter`/`Spec` with
SSH/DISPLAY availability checks. The abstraction hasn't converged, and adding
zenity pulls a dependency that pure-`creds` consumers shouldn't inherit — so it
waits until (a) the shape settles against a real migration and (b) we decide
whether it's a sub-package or its own module. The contract it must keep:
**secrets never transit argv**.

**Deferred — `redact`:** the redaction *mechanism* (tree-walk + `@redacted` +
`--expose`) is shareable with a CLI-supplied `shouldRedact` predicate, but only
~4 tools use it; below the rule of three for now.

## Validation plan

The settled design should be proven by migrating **one** tool before expanding.
`agent-cloudflare` or `agent-posthog` are the cleanest first targets
(single-profile, no parse-curl, keychain-managed). lin and slack come last
(lin's global-writer/truncation; slack's xoxc/cookie complexity). What the first
migration teaches — especially about the root builder's `Options` and the
config-defaults hook — feeds back here before `dialog` lands.
