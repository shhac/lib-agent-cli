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
| XDG `ConfigDir(app)` (in `xdg`, with spec env vars + fallbacks) | the app name |
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
   ├─ xdg   : ConfigDir/CacheDir/DataDir/StateDir/RuntimeDir (spec env + fallbacks) (+ App bundle)
   ├─ creds : Store (0600), Keychain, FirstNonEmpty/FirstNonZero/Getenv
   ├─ cli   : NewRoot(Options)+Globals, ConfigCommand, RequireConfirm, HandleUnknownCommand, Run
   └─ dialog: Prompter iface + PromptSecret/Prompt/Available + neutral Category/ClassifyError (the --form secret dialog)
```

`lib-agent-cli` depends on the **published** `lib-agent-output` tag, so a tool
that adopts both deletes its `internal/output/`, `internal/errors/`,
`internal/credential/`, `internal/config/` path logic, and most of
`internal/cli/root.go`.

## Scope and what's deferred

**In:** `creds`, the `cli` root builder, and `dialog` — the settled pieces
copied across the family.

**`dialog` (secret entry via native OS prompt) — the `--form` boilerplate.**
slack/posthog use a plain single `zenity.Entry`; cloudflare uses a richer
multi-field `Prompter`/`Spec` with SSH/DISPLAY availability checks; agent-sql
went furthest — a `Prompter` **interface** with a swappable `Default`, an
`InputType` enum, build-tagged + tested per-platform `Available`, and a
**neutral `Category`** taxonomy returned by `ClassifyError` so the package never
imports the host's error contract. We **adopted sql's design** as the family
dialog (v0.4.0): a multi-field `Spec` (slack's xoxc+xoxd needs two fields) with
a single-secret `PromptSecret` convenience on top, the pluggable `Prompter` seam
(+ a `dialogtest.Recorder` fake), `Field.Initial` prefill, and the neutral
`Category`/`ClassifyError` decoupling — the host maps `Category → fixable_by` in
~3 lines. Crucially the package does **not** import `lib-agent-output`, so it
drops into any sibling unchanged. The zenity dependency is acceptable here —
`lib-agent-cli` is the runtime lib and already carries cobra; a `creds`-only
consumer that never imports `dialog` doesn't compile zenity into its binary. The
load-bearing contract: **a secret never transits argv** — `--form` pops the
prompt instead.

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

## Persisted flag defaults are a boundary concern (July 2026)

A persisted default for a global flag (e.g. a config file's `defaults.format`)
is an instance of the family precedence `flag > env > persisted config >
built-in default` — it is **flag resolution**, not an output concern. The
settled pattern, proven by lin and adopted by agent-sql:

- The root's `ConfigDefaults` hook backfills the flag's `Globals` field when
  the flag is empty. Because the hook runs *before* `--format` validation, a
  bad persisted value gets the same structured error as a bad flag — no silent
  fallback.
- After pre-run, the `Globals` field is the single post-boundary truth. The
  output layer resolves it purely (parse-or-default) and **never reads the
  config store** — the anti-pattern this note exists to prevent is an emit-time
  `config.Read()` inside `internal/output` (agent-sql's original shape: impure,
  per-emit disk reads, validation bypassed).
- `ConfigDefaults` receives the command being executed so a CLI can scope a
  persisted default to a command class — e.g. agent-sql's `query.format`
  (allows csv) beside `defaults.format` (universal formats only), scoped via
  the same `AllowFormats` annotation that gates the validator, checked with
  `FormatAllowed`. One annotation is the source of truth for a domain format's
  whole reach: flag validity and default applicability.
