# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`locksmith` is a Go CLI (`github.com/deweysasser/locksmith`, `package main` at repo root) that catalogs SSH public keys and AWS access-key IDs discovered across local files, remote SSH hosts, and AWS accounts, then plans/applies changes to rotate or remove keys. See `README.md` for the user-facing command walkthrough.

## Common commands

```
make build        # go build .
make test         # go test ./...
make install      # go install (runs tests first)
make install-all  # cross-compile for darwin, windows, linux
make package      # build zip artifacts under dist/
```

Run a single test: `go test ./data -run TestStringSet` (replace package and test name).

`go vet ./...` and `gofmt -l .` are both clean — keep them that way. Both were brought to zero in
one pass, so any finding either one reports is something the current change introduced.

`make release` is a multi-step target that stashes work, checks out `main`, bumps `version.go`, commits, tags, and requires interactive `vi` for the changelog — do not invoke it casually; read `Makefile` first.

The module targets Go 1.27 (`go.mod` says `go 1.27.1`) and the `Dockerfile` uses `golang:1.27.1`; keep those in sync when bumping.

`github.com/aws/aws-sdk-go` (v1) is marked deprecated upstream in favor of `aws-sdk-go-v2`. The codebase still uses v1 throughout `connection/awsconnection.go` and `data/aws.go`; migrating to v2 is a non-trivial refactor, not a routine dep bump.

## Architecture

The system pipelines data from **connections** → **fetch** → **library storage** → **filter/display/plan/apply**.

### Packages

- **root (`main`)** — CLI wiring only. `main.go`, `commands.go`, `version.go`. Subcommand dispatch uses `github.com/urfave/cli` v1. Adding a subcommand means adding a `cli.Command` to `Commands` in `commands.go` and an `Action` handler in `command/`.
- **`command/`** — one file per subcommand (`fetch.go`, `list.go`, `connect.go`, `apply.go`, …). Shared helpers live in `command/common.go`: `datadir()` resolves the repo path (flag → `$LOCKSMITH_REPO` → `$HOME/.x-locksmith` → `$USERPROFILE/locksmith`), `buildFilterFromContext()` produces the substring-match filter used by every command, and `outputLevel()` maps flags to `output.Level`.
- **`connection/`** — five connection kinds implement `connection.Connection` (`Fetch(ctx) (<-chan data.Key, <-chan data.Account)`). The context is the only way out of a fetch: every implementation talks to something it does not control, and an implementation must both stop starting work when `ctx.Err() != nil` and guard every channel send with a `select` on `ctx.Done()`, or an abandoned fetch leaks a goroutine per producer. Both channels close whether the fetch completed or was cancelled: `FileConnection`, `SSHHostConnection`, `AWSConnection`, `GitHubConnection`, `DOConnection`. Only SSH and (partially) AWS implement `connection.Changer`; GitHub and Digital Ocean are **deliberately read-only** — GitHub permits key writes only against the authenticated user's own account, never another user's, and the Digital Ocean work was done under an explicit read-only constraint. `apply` skips a non-`Changer` connection with a warning. `SSHHostConnection.fetchSudo()` fans out work across `ParallelSSHCount` (default 5) goroutines per host.
- **`data/`** — domain types and the `Key`, `Account`, `Ider`, `Identiferser`, `Fetcher` interfaces. `FanInKey`/`FanInAccount` multiplex per-connection channels into a single stream for ingestion. `KeyBindingImpl` ties a key to an account at a `BindingLocation` (`AUTHORIZED_KEYS`, `CREDENTIALS`, `INSTANCE ROOT`, `FILE`).
- **`lib/`** — persistence. `library` (lowercase, `library.go`) is the generic store: one JSON file per object under `Path`, in-memory cache keyed by every ID returned by `Identifiers()`, type-dispatched deserialization via a `Type` field in the JSON and a global `TypeMap`. `MainLibrary` (`mainlib.go`) composes four typed wrappers — `KeyLibrary`, `AccountLibrary`, `ConnectionLibrary`, `ChangeLibrary` — stored under `keys/`, `accounts/`, `connections/`, `changes/` subdirs of the repo path.
- **`output/`** — leveled logger (`Error`/`Warn`/`Normal`/`Verbose`/`Debug`). Prefer these helpers over `fmt.Print*` so level gating works. `main.go` exits 1 when `output.ErrorCount()` is non-zero — but see *Known rough edges*: that counter does not currently count what its name says.

### Non-obvious invariants

1. **Register every new serializable type in `lib/mainlib.go` `init()`** via `AddType(reflect.TypeOf(...))`. Deserialization reads the `Type` string field from the JSON and looks it up in `TypeMap`; a missing registration *panics* (`library.deserialize`). Every `data.*` and `connection.*` type persisted to disk needs an entry here, and every struct so persisted must include a `Type string` field whose value matches the registered name.

2. **The typed libraries are generics, not code generation.** `lib/typedlibrary.go`
defines one `TypedLibrary[T]` over the generic `library`, and `KeyLibrary`,
`AccountLibrary`, `ConnectionLibrary` and `ChangeLibrary` are aliases for
instantiations of it. There is no `lib/Makefile` and no `go:generate`; the four
`sed`-generated files are gone. The one thing the template could not express --
that `data.Change` is a value type, so it arrives from disk as a pointer and
from the cache as a value -- is now the `coercion[T]` argument each constructor
supplies (`asInterface` for the interface element types, `asChange` for
`data.Change`). Adding a library kind means one constructor, not a new file.

3. **Ingestion never deletes a key; it deletes bindings.** A key record in
`keys/` is a permanent catalog entry -- it keeps its names, comments and
first-seen date even when nothing references it any more, and "this key exists
and is bound nowhere" is a meaningful answer rather than an artifact. Only
`locksmith remove` deletes keys, and only when the user names a filter. (The
`klib.Delete` in `command/fetch.go` is not a deletion: it is the rename that
happens when a fingerprint-only key later gains real public key material and so
changes primary ID.)

4. **A connection may only claim authority over a binding location it
enumerated in full.** `accountImpl.Observed`, set via `MarkObserved`, tells
`mergeBindings` which locations the observation is *complete* for; bindings
recorded at those locations and not seen this time are dropped. Claiming a
location that was merely sampled silently deletes real inventory. Today SSH
claims `AUTHORIZED_KEYS` (it reads the whole file) and GitHub claims it (the
endpoint returns the user's whole key set). AWS deliberately claims nothing:
`fetchKeyPairs` emits one `AWSAccount` **per region**, all merging onto the same
ARN, so each is complete for its region and partial for the account -- claiming
there would delete 29 regions' worth of bindings. DO droplet accounts likewise
claim nothing, because DO never reports droplet-to-key linkage at all.

   The corollary is that connections must report an account **even when it has
no keys**, or an emptied `authorized_keys` could never clear what was recorded.
`command.ingestAccounts` is what declines to store a *new* keyless account, so
the inventory does not fill with system accounts that will never hold a key.

5. **`data.Change.Id()` must stay on a value receiver.** Changes are stored, listed and deleted *by value*, and a pointer-receiver method is not in the method set of the value type — so `library.Id()` would stop seeing a `Change` as a `data.Ider` and silently fall back to hashing the JSON. That keys changes by content instead of by account, and re-running `plan` after anything changes then leaves a stale change file beside the new one instead of replacing it. `lib.TestChangeLibraryKeepsOneChangePerAccount` and `command.TestCalculateChangesReplacesAStalePlan` both fail if this is changed back.

6. **Key-algorithm dispatch goes through `data.IsPublicKeyAlgorithm`, never a substring test.**
`NewKey` used to decide "this is a public key" with `strings.Contains(content, "ssh-")`, which
catches `ssh-rsa`/`ssh-dss`/`ssh-ed25519` by spelling alone and silently dropped every
`ecdsa-sha2-*` and `sk-*` key. The algorithm list in `data/keytypes.go` is built from
`x/crypto/ssh`'s own `KeyAlgo*` constants so it tracks the library. Note the shape check scans
*every* field on a line, not just the first two, because an option value may contain quoted
whitespace (`command="/bin/ps -ef"` splits into two fields on its own).

7. **The persisted `Type` field is set by hand at every construction site.** It is an ordinary
struct field, not something the library fills in — `data.Change{Type: "Change", …}`,
`connection.FileConnection{Type: "FileConnection", …}`. A new persisted type needs the field,
a matching `AddType` registration, and the right literal at every place it is constructed.

### Key lifecycle: connect → fetch → expire → plan → apply

The interesting flow spans four files and is not obvious from any one of them.

1. **`connect`** (`command/connect.go`) — `NewConnection()` picks the connection type from the
   argument's shape: an existing filesystem path → `FileConnection`; `aws:PROFILE` →
   `AWSConnection`; `gh:USERNAME` → `GitHubConnection`; `do:NAME` → `DOConnection` (credential
   comes from `$DIGITALOCEAN_ACCESS_TOKEN`, the name is only a label); anything else →
   `SSHHostConnection`, with sudo turned on automatically for `ubuntu@`/`root@`/`ec2-user@`
   targets unless `--no-sudo`. The connection is persisted; nothing is contacted yet.

   Adding a connection kind means touching two shared places, which is where parallel work
   collides: the `switch` in `NewConnection()` and the `AddType` block in `lib/mainlib.go`.
2. **`fetch`** — runs every stored connection's `Fetch()`, fans the key and account channels in
   (`data.FanInKey`/`FanInAccount`), and merges results into the key and account libraries.
3. **`expire`** (`command/expire.go`) — sets `Deprecated` on every key matching the filter. This
   only marks the key in the local repo; it touches no remote system. A key may also carry a
   `Replacement` ID (see `keyImpl` in `data/keys.go`) meaning "wherever this key is bound, bind that
   one instead".
4. **`plan`** (`command/plan.go`) — `calculateChanges()` walks every account's bindings, looks each
   bound key up in the key library, and materializes a `data.Change` per account: deprecated keys
   become `Remove` entries, keys with a `Replacement` become `Add` entries. Changes are persisted to
   `changes/`, so `plan` is cumulative across runs, not a fresh computation each time.
5. **`apply`** (`command/apply.go`) — for each stored change, resolves account →
   `account.ConnectionID()` → connection, type-asserts it to `connection.Changer`, and calls
   `Update()`. A connection that isn't a `Changer` is skipped with a warning. The change is deleted
   from the library only on success, so a failed apply stays pending. `SSHHostConnection.Update()`
   adds keys before removing any, and aborts the removal if the addition failed — don't reorder
   that.

**`add`** is the exception: it writes `Change` objects straight to the change library, bypassing
`plan` entirely. `plan` will not regenerate them, and `apply` consumes them like any other change.

### Filtering model

Every non-trivial command accepts positional args that become substring filters. `buildFilter` (in `command/common.go`) returns a predicate that stringifies each candidate object via `fmt.Sprintf("%s", i)` and accepts the object if *any* arg is a substring — filters combine as **union**, not intersection. The filter is applied against the same representation the object would render in `list` *without* `-v`, so verbose-only fields are not filterable.

### Known rough edges

These are real, verified, and *not* fixed. Don't rediscover them; don't mistake them for something
you broke.

- **`locksmith` exits 1 on every invocation.** `output.output()` gates the counter increment on
  `if l >= ErrorLevel`, and `ErrorLevel` is `iota` = 0 — so every leveled call increments it,
  including `Debug` calls suppressed by the level gate. `datadir()` calls `output.Debug` before any
  subcommand does its work, so the count is non-zero before anything happens and `main.go` exits 1.
  Fixing it means gating on the message level actually being `ErrorLevel`.
- **`output.ErrorCount()` closes `errorChannel`.** Calling it twice, or any output call after it,
  panics on send-to-closed-channel. It is safe only as the last thing `main` does.
- **`--silent` is unreachable.** `outputLevel()` checks `c.Bool("silent")` and
  `c.GlobalBool("verbose")`, but `commands.go` registers neither: `GlobalFlags` is only
  `debug`/`repo`, and `verbose` exists solely as a per-command flag. `SilentLevel` cannot be
  selected from the CLI.
- **`version.go` says 0.11; the newest tag is `release/0.9`.** The `make release` bookkeeping and
  the tags have drifted apart.
- **`README.md` describes a `master`/`development`/`prototype` branch layout.** The default branch
  is `main`; `origin/master` does not exist.

### Data at rest

`~/.x-locksmith/` (the leading `x-` is intentional; storage format is still considered unstable per README). Objects are individual JSON files, safe to commit to git. Private keys and AWS secret-key material are never written — only public keys, fingerprints, and access-key IDs.

### Tests

`go test ./...` is clean, as is `go test -race ./...`. Tests that touch the
filesystem use `t.TempDir()`; the older tests in `lib/` share a `test-output/`
directory instead and are therefore order-dependent, so prefer `t.TempDir()` for
anything new. Tests that would otherwise print through `output/` call a local
`silence` helper — note that `output.Error` ignores the level and always prints,
per *Known rough edges*.

The `data` and `lib` packages are the ones worth keeping well covered: they hold
the merge, identity and persistence rules that everything else relies on.
Coverage of `connection/` and the `Cmd*` entry points is necessarily thin, since
they are the parts that talk to SSH and AWS.

### Test fixtures

`data/test-data/` holds real-format SSH private keys (`rsa`, `dss`, `constrained`), a `*.pem`, and
an AWS-style `credentials` file. They are throwaway fixtures generated for the test suite, not live
secrets — a security pass will flag them, and that flag is a false positive. Tests read them by
relative path, so they run from the package directory.
