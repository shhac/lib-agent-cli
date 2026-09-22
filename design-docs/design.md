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

## A config a human edits is not a credentials file (September 2026)

`creds.Store` was built for credentials: a small document written wholly by
its struct, where `Save` marshalling the struct and replacing the file is
exactly right. Config files reached for the same store, and inherited a rule
that does not fit them.

Two things a config has that a credentials file does not. It carries
**comments** — JSON has none, so the family writes them as `"//"` keys, the
npm convention — and it **outlives the release that wrote it**, so it can hold
a key only a newer build understands. A struct has a field for neither, so a
load-modify-save dropped both: changing one unrelated setting stripped a
config's entire documentation, and silently discarded a setting the next
release would have read. Consuming CLIs did not notice, because nothing errors
— the file simply comes back smaller.

`Store.Overlay` lays the struct's view over the stored document rather than
replacing it. The rule that makes it precise rather than a guess is the
schema: a key the struct's json tags OWN but the value no longer states was
unset and must go, while a key with no field was never ours to remove. Without
that distinction the merge would preserve everything and `unset` would
silently stop working.

Two decisions inside it are worth recording because they are not forced:

- **Map entries are owned, their contents are not.** The struct round-trips a
  whole map, so an entry it no longer lists was deleted. Inside an entry the
  element's schema applies again, so a stray key there survives.
- **Arrays replace wholesale.** Their elements have no identity to merge on,
  so the struct's list is the answer. This is the one place an unrecognised
  key does not survive, and it is a choice rather than an oversight.

Layout is a sort rather than a preserved order: a note sorts as the key it
documents, ties breaking note-first, so a pinned pair is indivisible and
nothing lands between a comment and its subject. Order is then a pure function
of the key set — the same content writes the same bytes, and a note dropped
anywhere in an object migrates beside its key on the next write. Preserving
the input order instead was considered and rejected: it is faithful to
whatever the file already contains, where a sort is self-healing.

It is **opt-in**. Preserving an unrecognised key in a credentials store would
be a way to keep a secret alive past the code that knew about it, which is the
opposite of what that store is for.

## Getting at a key the registry does not have (September 2026)

`Overlay` stops a save destroying a key the schema cannot see, but preserving
one is not surfacing it: a typo, a key from a newer build, and a key this
release renamed all behave exactly like a key nobody ever wrote. The setting
is not in effect and nothing says so — and `config get`, the one command that
could show what it holds, answers "unknown config key", which is true of the
schema and unhelpful about the document.

`creds` gained the reads (`UnknownKeys`, `RawValue`, `RawDelete`) and `cli`
the wiring (`ConfigCommand(..., WithDocument(store, schema))`), which extends
`get` and `unset` to keys the document holds and the registry lacks. Three
boundaries worth recording:

- **`set` is not extended.** A key nothing reads is not a setting, and writing
  one would manufacture the state this exists to clear.
- **A typo still reaches the library.** A name in neither the registry nor the
  document keeps its "unknown config key" error and the list of valid names,
  which is what a typo actually needs. Only a name the document *holds* takes
  the fallback.
- **Opt-in, and it has to be.** `ConfigCommand` knows the key closures, not
  the store behind them, so it cannot do this unasked. A CLI that wants its
  own behaviour passes no option and nothing changes.

The discriminator is the registry, not the library's error class. An earlier
version in a consuming CLI classified `FixableByAgent` as "unknown key", but
`ConfigCommand` returns that same class when a KNOWN key's `Unset` fails — so
a failed unset of a key whose CLI name matches its file path would fall
through and delete it from the document, reporting as success the write the
library had just refused.

`SectionKey` covers the other half: `unset` on a group of keys, to put a whole
section back to defaults. Clearing must go through the struct — deleting the
section from the document alone would last until the next save wrote it back —
so the clearing closure is the CLI's, and the library supplies only the shape:
`set` explains that a section is not a value rather than reporting it
read-only, which it is not.

## The document algebra is not credential code (September 2026)

`creds` is the one audited place for `0600` permissions and keychain handling,
and the overlay work grew it a second job: the `"//"` note convention, the
layout sort, the schema-aware merge and the unknown-key walk. By the time it
shipped, most of the package's code never touched a secret or a file, which
dilutes exactly the audit the package exists to make cheap.

That half now lives in `internal/jsondoc`. It is pure (decoded maps and
reflect types in, maps out), so its tests need no filesystem, and a reviewer
of `creds` reads only the store, the lock, the atomic write and the thin
methods that hand a decoded document across. It is `internal` because it is
an implementation of `Store.Overlay`, not a surface: `creds` converts its
results into its own exported types (`UnknownKey` is a `creds` struct, not an
alias), so the package can be reshaped without touching any consuming CLI.
