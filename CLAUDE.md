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

`make release` is a multi-step target that stashes work, checks out `master`, bumps `version.go`, commits, tags, and requires interactive `vi` for the changelog — do not invoke it casually; read `Makefile` first.

The module targets Go 1.26 (`go.mod`) and the `Dockerfile` uses `golang:1.26.0`; keep those in sync when bumping.

`github.com/aws/aws-sdk-go` (v1) is marked deprecated upstream in favor of `aws-sdk-go-v2`. The codebase still uses v1 throughout `connection/awsconnection.go` and `data/aws.go`; migrating to v2 is a non-trivial refactor, not a routine dep bump.

## Architecture

The system pipelines data from **connections** → **fetch** → **library storage** → **filter/display/plan/apply**.

### Packages

- **root (`main`)** — CLI wiring only. `main.go`, `commands.go`, `version.go`. Subcommand dispatch uses `github.com/urfave/cli` v1. Adding a subcommand means adding a `cli.Command` to `Commands` in `commands.go` and an `Action` handler in `command/`.
- **`command/`** — one file per subcommand (`fetch.go`, `list.go`, `connect.go`, `apply.go`, …). Shared helpers live in `command/common.go`: `datadir()` resolves the repo path (flag → `$LOCKSMITH_REPO` → `$HOME/.x-locksmith` → `$USERPROFILE/locksmith`), `buildFilterFromContext()` produces the substring-match filter used by every command, and `outputLevel()` maps flags to `output.Level`.
- **`connection/`** — three connection kinds implement `connection.Connection` (`Fetch() (<-chan data.Key, <-chan data.Account)`): `FileConnection`, `SSHHostConnection`, `AWSConnection`. SSH and (partially) AWS additionally implement `connection.Changer` so `apply` can mutate remote state. `SSHHostConnection.fetchSudo()` fans out work across `ParallelSSHCount` (default 5) goroutines per host.
- **`data/`** — domain types and the `Key`, `Account`, `Ider`, `Identiferser`, `Fetcher` interfaces. `FanInKey`/`FanInAccount` multiplex per-connection channels into a single stream for ingestion. `KeyBindingImpl` ties a key to an account at a `BindingLocation` (`AUTHORIZED_KEYS`, `CREDENTIALS`, `INSTANCE ROOT`, `FILE`).
- **`lib/`** — persistence. `library` (lowercase, `library.go`) is the generic store: one JSON file per object under `Path`, in-memory cache keyed by every ID returned by `Identifiers()`, type-dispatched deserialization via a `Type` field in the JSON and a global `TypeMap`. `MainLibrary` (`mainlib.go`) composes four typed wrappers — `KeyLibrary`, `AccountLibrary`, `ConnectionLibrary`, `ChangeLibrary` — stored under `keys/`, `accounts/`, `connections/`, `changes/` subdirs of the repo path.
- **`output/`** — leveled logger (`Error`/`Warn`/`Normal`/`Verbose`/`Debug`). Any `Error*` call increments a counter; if non-zero at exit, `main.go` exits 1. Prefer these helpers over `fmt.Print*` for user-visible output so debug/verbose gating and the error-count behavior both work.

### Two non-obvious invariants

1. **Register every new serializable type in `lib/mainlib.go` `init()`** via `AddType(reflect.TypeOf(...))`. Deserialization reads the `Type` string field from the JSON and looks it up in `TypeMap`; a missing registration *panics* (`library.deserialize`). Every `data.*` and `connection.*` type persisted to disk needs an entry here, and every struct so persisted must include a `Type string` field whose value matches the registered name.

2. **`lib/{account,change,connection}Library.go` are generated from `keyLibrary.go`** by `lib/Makefile` using `sed` substitutions (there is a `//go:generate make` directive in `keyLibrary.go`). Do not hand-edit those three files — change `keyLibrary.go` and re-run `make -C lib` (or `go generate ./lib/...`). The `connectionLibrary.go` rule additionally adds a `connection` import.

### Filtering model

Every non-trivial command accepts positional args that become substring filters. `buildFilter` (in `command/common.go`) returns a predicate that stringifies each candidate object via `fmt.Sprintf("%s", i)` and accepts the object if *any* arg is a substring — filters combine as **union**, not intersection. The filter is applied against the same representation the object would render in `list` *without* `-v`, so verbose-only fields are not filterable.

### Data at rest

`~/.x-locksmith/` (the leading `x-` is intentional; storage format is still considered unstable per README). Objects are individual JSON files, safe to commit to git. Private keys and AWS secret-key material are never written — only public keys, fingerprints, and access-key IDs.
