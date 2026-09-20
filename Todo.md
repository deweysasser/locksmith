TODO

## Current
- [x] Implement support for Ed25519 keys
      (already worked; the real bug was that `ecdsa-sha2-*` and `sk-*` keys were
      silently dropped by the `"ssh-"` substring dispatch -- now fixed)
- [x] Load keys from a named user/account in github  (`gh:USERNAME`, read-only)
- [x] load keys form a named user/account in digital ocean  (`do:NAME`, read-only)
- [x] connect to digital ocean droplets to survey keys in a similar way as we do for AWS
      (same `do:` connection; note DO does not report which keys a droplet was built with)

### PR

- [ ] Implment a method to avoid over-fetching from github -- that will likely require maintaining a state file in the data directory.  assume that the state file will NOT be checked in.
- [ ] When creating the data directory for the first time, create a .gitignore to avoid the rate limit record file above

### PR

- [ ] when keys are discovered via a file connector, they should be recorded with the current hostname.   File connector may be valid on multiple hosts, but we still need to track where.

### PR

- [ ] when running against a user .ssh/ path, if it contains an `authorized_keys` file, that should also be included

### PR

- [ ] Upgrade to latest AWS API library version

### PR

- [ ] Support ingesting keys from 1password, either by specific ID or all of a 1password account.  Keys in 1password should NEVER be deleted.  If they are deprecated or expired a line should be added to the key's "note" field about that.
- [ ] Support reading AWS credentials from 1password

### PR

- [ ] Change the command protocol.
  - instead of "connect", it should accept "connection" or "conn" or "c" with subcommands add, list|ls, del|delete|rm|remove
  - take out "list"
  - implement "keys|key|k" command with subcommands "add", "list|ls", "del|delete|rm|remove" , "expire", "add-id"
  - "add" should mark a key to be added to a system
  - "del|delete|rm|remove" should mark a key to be removed from a system
  - "replace <old> <new>" should mark things to replace the old key with the new key on all systems or any system discovered in the future

### PR

- [ ] Pace requests against provider rate limits.  This is *not* a concurrency
      problem and a worker pool is the wrong tool: a cap bounds calls in flight,
      while a rate limit is calls over time.  Ten concurrent GitHub requests is
      fine if you make ten and fatal if you make 600 in an hour.  Every provider
      already tells us the budget, so read it rather than guess a number:
  - github: 60 requests/hour **per IP** when unauthenticated, which is the one
    that bites immediately -- twenty connected users is a third of the hourly
    budget in a single fetch.  `githubForbiddenError` already parses
    `X-RateLimit-Remaining` and `X-RateLimit-Reset`, but only to explain the
    failure after the budget is gone.  Use the same headers to pace beforehand.
  - digital ocean: 5000/hour, 250/minute burst.  `godo` parses `Rate` off every
    response and we discard it; cheapest of the three to fix.
  - aws: nothing to do.  The v1 SDK's default retryer already backs off on
    throttling errors, and hand-rolled pacing would fight it.
  - state belongs per provider, not global, because the limit is per token and
    per IP rather than per process.

- [ ] Bound SSH fan-out across hosts.  Unrelated to the item above despite
      looking similar: SSH has no API budget, so this is about local resources
      and about not hammering a shared bastion.  Each SSHHostConnection forks
      ParallelSSHCount+1 = 6 ssh processes, so ~170 hosts in flight exhausts a
      default 1024 fd limit.  ParallelSSHCount already bounds work *within* a
      host; what is missing is a cap *across* them.  Needs its own flag, and
      needs care with the fan-in lifecycle, which requires every Add before
      Wait -- that is why it was not done alongside the context work.

### PR
- [ ] Resolve AWS credentials the way the AWS CLI does.  `awsconnection.go`
      builds them with `credentials.NewSharedCredentials`, which reads
      **only** `~/.aws/credentials` and ignores `~/.aws/config` entirely, so
      config-file profiles, SSO, assume-role and `credential_process` cannot be
      used at all.  Replace it with
      `session.NewSessionWithOptions{Profile, SharedConfigState: SharedConfigEnable}`
      in both the main and per-region paths; the region then comes from the
      profile instead of the hardcoded `us-east-1`.  Note this changes
      behaviour for anyone relying on that override.  The service-interface
      stub tests are unaffected.
      Also improve the error: `SharedCredsLoad: failed to get profile` gives no
      hint that the config file was never consulted.  Say which files were
      searched and for what profile name.

- [ ] Support `aws login` credentials (CLI v2).  Separate from the item above
      and not fixed by it -- verified by testing.  `aws login` acquires console
      credentials into its **own** cache (overridable via
      `AWS_LOGIN_CACHE_DIRECTORY`) and writes a profile like:

          [PROFILE]
          login_session = arn:aws:iam::ACCOUNT:user/NAME
          region = us-east-1

      `login_session` is not a credential source any SDK understands, so
      aws-sdk-go v1 and v2 both fail with NoCredentialProviders.  Options, in
      increasing order of effort: rely on the default credential chain and have
      the operator export credentials into the environment; or teach locksmith
      to read the login cache directly, which needs its format pinned down
      first.
      Worth deciding whether this is locksmith's job at all, or whether the
      answer is simply "use a profile the SDK can resolve".

## Open from the 2026-09 review

Found by the six standard reviewers and not yet done.  Several dissolve into
the command-protocol redesign above, so check that first rather than fixing
them into a shape that is about to change.

### Correctness

- [ ] Separate desired state from observed state.  Intent is stored *on the
      observed object* -- `expire` sets `Deprecated` on the key record itself --
      so it cannot be revoked, there is no `unexpire`, and `Merge` only ever ORs
      the flag on.  Proposal: a policy object per key
      (`{KeyID, Disposition: Remove|Replace|Require, Replacement, Scope}`), with
      `plan` becoming a pure function of (policy, inventory).  That gives
      `unexpire` for free (delete a file), unifies `add` with `expire`, and
      gets "remember global key additions so they apply to servers added later"
      as a side effect.  Note the convergence half of this is already done:
      bindings now drop when a fetch asserts it observed a location in full.

- [ ] `plan` is an accumulator, not a diff.  It never deletes a change that is
      no longer warranted, so stale changes persist in `changes/`, in `list`,
      and in every subsequent `apply`.  It should regenerate the directory
      wholesale.

- [ ] `plan` silently destroys a change created by `add`.  Both write a Change
      keyed by account ID, so they collide on the same file and `plan` wins with
      no warning -- and `plan` is exactly what a user runs next to inspect what
      they just asked for.

- [ ] `plan` emits changes that can never be applied.  Only SSH implements
      `connection.Changer`; file, AWS, GitHub and DO do not.  Capability is
      discovered at the last moment by a type assertion inside `apply`, so
      changes accumulate for accounts nothing can act on, are never deleted (that
      happens only on a successful Update), and re-warn on every run.  Give
      `Connection` a capability query and have `plan` either skip those accounts
      or mark the change advisory.

- [ ] `Earliest` means different things depending on source.  A file connection
      records the file's mtime; GitHub and AWS record when locksmith first saw
      the key, because neither API offers a creation date.  So a five-year-old
      GitHub key reads as new and sorts as newer than a local file key -- which
      is actively misleading if you hunt stale keys by age.  Separate "first
      seen" from "created", and show whichever is known.

### Robustness

- [ ] `SshCmd` has no timeout, never drains stderr, and its framing is forgeable.
      `Run` blocks in `ReadLine` with no deadline (there is a standing TODO), so
      a host that accepts the connection and goes quiet hangs the run -- the
      fetch context does not reach inside it.  The unread stderr pipe blocks the
      writer once the 64KiB kernel buffer fills, and a failing remote command
      reports `Non-zero exit: 1` while discarding the message that would explain
      it.  The command boundary is a fixed string matched as a prefix rather
      than a per-command nonce matched as a whole line, so remote content can
      be mistaken for it and desynchronise the session.  Use a per-command
      nonce, match the whole line, drain stderr into a bounded buffer, and add
      a deadline.  (Impact analysis deliberately kept out of this file; see the
      note below on disclosure.)

- [ ] `apply` cannot report failure.  `CmdApply` returns nil unconditionally;
      every per-host failure is logged and skipped.  For a tool whose job is
      revocation, a cron or CI wrapper cannot tell "revoked everywhere" from
      "revoked nowhere" without scraping stdout.  Accumulate failures and return
      an error.

- [ ] `apply` has no test at all, because its logic lives inline in `CmdApply`
      and needs a `*cli.Context` and a real repo.  Extract `applyChanges(...)`
      the way `calculateChanges` already is, and test the three invariants
      CLAUDE.md calls load-bearing: the change is deleted only on success, a
      non-Changer connection leaves it pending, and add precedes remove.

- [ ] Persistence errors are discarded across `command/`.  `Store`, `DeleteObject`
      and `Flush` drop their returned error in roughly fifteen places
      (`plan.go`, `add.go`, `connect.go`, `expire.go`, `apply.go`, `addid.go`).
      A read-only repo or a full disk means `plan` prints a plan it did not save
      and exits 0.  The sharpest is `apply.go`: a failed `DeleteObject` means a
      change that *was* applied stays pending and is applied again next run.
      `fetch.go` already checks -- the habit exists, it is just not applied.

- [ ] Panics reachable from repository content.  `library.deserialize` panics on
      valid JSON with a missing or unknown `Type`, and `command/fetch.go` panics
      on an object that is not a Connection or Account.  The README invites
      teams to hand-merge this repository in git, so a bad merge or a version
      skew takes the tool down for everyone -- including for the `apply` that was
      meant to revoke someone's access.  Malformed JSON is already skipped
      silently, which is exactly backwards: the recoverable case is fatal and
      the data-loss case is quiet.  Report and skip; warn on the skip.

- [ ] `lib.sanitize` is not injective.  `\W+ -> _` maps `alice@a.b.com` and
      `alice@a-b.com` to one filename, and the second `Store` silently
      overwrites the first.  Within a run the cache keeps them apart, so the loss
      only appears on the *next* run.  Appending a short hash of the raw ID fixes
      it but changes the on-disk format; verifying the stored ID on read has no
      migration cost and at least turns silent corruption into an error.

### Structure

- [ ] `DOAccount` and `DODropletAccount` live in `connection/`, while every other
      account type lives in `data/`.  The cost is already visible:
      `doMergeBindings` and `doBindingChannel` reimplement `data.mergeBindings`
      and `accountImpl.Bindings`, so a fix to binding merge has to be made twice
      by someone who knows both exist.  Moving them keeps the persisted `Type`
      strings, or existing repositories stop deserializing.

- [ ] `data/` depends on the AWS SDK in its public API -- `NewIAMAccount`,
      `NewIAMAccountFromKey` and `NewAWSInstanceAccount` take `*iam.User`,
      `*iam.AccessKeyMetadata` and `*ec2.Instance`.  So the core domain package
      links `aws-sdk-go`, and the eventual v2 migration churns `data/` as well as
      `connection/`.  The DO code shows the alternative and gets it right: plain
      values in, `godo` never mentioned in `data/`.

- [ ] `output` writes everything to stdout, including errors and warnings, so
      `locksmith list | grep` swallows the error text and any consumer gets it
      interleaved with data.  Errors and warnings belong on stderr.  While there:
      `SilentLevel` is the level at which *warnings* print and should be called
      `WarnLevel`; `OutputLevel` stutters; `IsLevel(l)` reads as "is the level"
      but means "is enabled"; and the `Errorf(fmt string, ...)` parameter shadows
      the imported `fmt` package.

### Tests

- [ ] Assertions that cannot fail.  `lib/library_test.go:verifyMultiIDs` compares
      a `[]data.ID` against a `*multiID` -- never equal, and the sense is inverted
      besides, so the multi-identifier cache test only checks that Fetch returns
      no error.  `lib/keyLibrary_test.go:Test_keyLibrary_Basic` runs its body only
      when Fetch *fails*.  `command/common_test.go:TestKeyAndAccountFilter_Wrap`
      checks a func value is non-nil.  Three tests in `command/list_test.go` call
      the printers with output silenced and assert nothing.

- [ ] The older `lib/` tests share a `test-output/` directory rather than
      `t.TempDir()`, so they are order-dependent and cannot run with `-shuffle`.
      `data/ssh_test.go` also writes two files into the package directory that
      nothing reads, one of them with mode `666` decimal.

- [ ] `connection/testdata/fakessh-sandbox` rewrites every `~user/` to one
      directory, so "wrote to the wrong user's authorized_keys" is undetectable --
      and the sudo path exists precisely to edit other users' files.

- [ ] Two functions named `SkipTestSSHPrivateKeyParse` and `skipTestAWSIds` never
      run, and were the only assertions for private-key fingerprints and the AWS
      PKCS8 ID.  Either restore or delete them.

- [ ] Helper sprawl now that the parallel branches have landed: five near-identical
      output silencers that disagree on level, two stdout capturers, four channel
      collectors, three ID-to-sorted-string converters.  The distinct prefixes did
      their job and can retire into one shared helper file.

### Dead code

- [ ] `getAWSID` and `asHex` in `data/ssh.go` compute the AWS-format fingerprint
      that `add-id` exists to supply by hand, but the call site is commented out
      next to a TODO saying the API changed.  Either finish the feature or delete
      the stub and keep `add-id` as the documented answer.  Note an Ed25519
      branch would need `MarshalPKCS8PrivateKey`, which is a different hash input
      than the RSA/ECDSA branches use.

- [ ] Unreferenced: `publicKey`, `getId`, `LoadJsonFile`, `LoadTypeFromJSON`,
      `data.Read` (test-only), `mergeIDArrays`, `lib.keyid`, `lib.keyReadError`,
      `lib.IdStringer`, the whole `connection/remote.go`, `connection/test-utils.go`
      (a bare package clause), `data.KeyBinding`, and `library.Flush`, which is a
      no-op that `add-id` defers as though it mattered.

## Old

* add 'version' command
* add some kind of 'info' command that shows repo location, SSH, etc.
  Perhaps mirror "go env"
* make sure `locksmith remove` doesn't just remove everything
* 'plan' changes automatically as they're discovered
* recursive fetch from credentials file (but only for keys that
  haven't been deprecated)
* recursive fetch for instances (but only if the key is loaded)
* implement object tags?
* filters should accept regular expressions
* improve on sudo error handling (detect when we can't sudo)

* decorate AWS accounts with the profile name used to reach them if
  three are no aliases
* decorate AWS Iam accounts with the account alias of the AWS account
* decorate AWS instances with the AWS account to which they belong

* remove should remove changes as well

* Create github release process
* compute AWS format fingerprints of private keys

* fetch -v should show the accounts/keys it creates

* deal with "Failed to parse key" error message

* improve SSH root handling to avoid creating so many connections

* implement "recursive connect" for AWS (i.e. SSH to all of the
  instances with public DNSes)
  - How do we determine login account name?
  - Should we check the current ssh-agent to see if it has the key
  loaded or if the key is otherwise available?
  
* implement "recursive connect" for AWS credentials file
* remove accounts for instances that have been terminated during AWS fetch

* can we somehow detect gitolite so we don't do the usual thing with
  those keys?

* handle pkcs8 format public keys?  ssh-keygen -f <key> -y -e -m PKCS8

* record hostname when recording key name by path.  Should we record
  the entire path?

* should comment and names share the same namespace?

* Move SSH fetch to pure golang implementation (must work with OpenSSH agent)
* Make SSH fetch work with PLINK on Windows
* document commands

* more complicated filters
* handle key options in bindings
* mirror terraform's plan/apply for changes
* remove keys from accounts via connections
* replace a key wherever it is
* deprecate a key wherever it is
* Do something with key aging

* Rotate SSH key in AWS

* import SSH keys from github
* replace SSH keys in github

* import SSH keys from GCP
* replace SSH keys in GCP

* import SSH keys from DO
* replace SSH keys in DO

* generate AWS key for a given user
* remove given AWS key
* rotate given AWS key

* change key comment everywhere???

* Handle AWS accounts (not keys)???
* AWS key age/expiration
* parse the private key to generate the #$%#$%!@#$ Amazon fingerprint crap.  Must handle encrypted private keys.  Amazon has destroyed more productivity with this stupid decision than should be tolerated.

* parse multiple keys from a local file (e.g. known hosts)

MAYBE

* "find" things by filtering on not top level


DONE
* remove a key/connection/account/whatever
* AWS iterate over all regions
* verbose listing which shows keys for account (and accounts for key?)
* record flie name as key 'Name'
* record AWS name as key 'Name'
* display key names on listing
* display keys by account
* display accounts by key
* Import SSH keys from AWS
* import AWS keys from file
* report on instances using SSH key (can we build ssh access URLs?)
* import AWS keys from AWS account
* test on MacOS
* allow specification of different key repo location
* Do some scale testing
* make an SSH key placeholder with just a fingerprint (Note:  how do we handle changing IDs when we finally discover a public key for it?)
* be able to handle key references with no public key (e.g. when  Amazon has a key fingerprint we don't recognize)
* make IAM accounts use ARN or alias
* implement "recursive connect" for SSH (i.e. sudo)
* Add a docker build file
* automatically try sudo when we're 'root' or 'ubuntu' or 'ec2-user'
* Grab AWS keys from AWS, relate to user accounts
* Record AWS account ID# and alias
* Don't record accounts for which we have no keys (?)
* Fix the !@#$%# AWS Fingerprints!
* detect username on system for when we don't have a username

## Open from the 2026-09 review, round 2

Round-2 findings that were *not* fixed in this branch. Everything the reviewers
rated HIGH was fixed; these are the MEDIUMs and LOWs that need more than a
local edit.

### The comment on a rendered line is attacker-choosable

(The restriction-dropping half of this finding is fixed: options now ride on
`KeyBindingImpl.Options`. What follows is the part that is not.)

`GetSshLine` picks `Comments.StringArray()[0]`, and `StringSet` sorts,
so among every comment ever merged for a key the lexicographically smallest
wins. That is attacker-choosable by anyone who can publish a key locksmith
surveys. Choose the comment deterministically from the binding instead.

### History write failures are silent, and apply proceeds anyway

`history.Log.Record` returns an error and all six call sites drop it, as does
`defer log.Close()`. `Log.open` caches its failure, so once the first write
fails every later one fails identically and invisibly for the rest of the run.
`apply` writes the record *before* `Changes().DeleteObject`, so on a read-only
checkout or a full disk it removes keys from every host, deletes the pending
changes that would have reconstructed what it did, prints nothing, and exits 0.

Fix: check the return, report it through `output.Error`, and have `apply`
refuse to delete the change when the record of the work could not be written.
This is adjacent to the filed "Persistence errors are discarded across
`command/`" item but not covered by it -- that one enumerates `Store`,
`DeleteObject` and `Flush` and does not reach `history`.

### Cancellation is still missing on three connections, and one goroutine leaks

The `ctx.Done()` guard on channel sends reached SSH and `FileConnection` but not
`connection/awsconnection.go` (eight sites), `connection/githubconnection.go`
(two) or `connection/doconnection.go` (three). It is not a hang today only
because `CmdFetch` never stops draining; any caller that abandons a `Fetch`
leaks a goroutine per producer, and AWS starts one per region. The
`sendKey`/`sendAccount` helpers in `connection/ssh.go` are already written and
package-scoped.

One real leak survives inside the SSH path: `retreiveSystemUsers`' goroutine
sends on `users` with no guard, and its consumers return early both on
`ctx.Err()` and on `NewSshCmd` failure -- so on cancellation, or when fd
exhaustion makes all five workers fail to connect, it blocks forever holding
the host's entire `getent passwd` output.

Also `NewSshCmd` returns `nil, err` from `StdoutPipe`/`StderrPipe` without
closing the pipes it already made, which is exactly the failure that happens
under fd pressure.

### Allocation and syscall costs, in rough order of payoff

None of these is what limits the tool today; they are filed so the numbers
exist when fan-out gets bounded.

- `connection/fileconnection.go` -- `Fetch` walks the whole tree synchronously
  and leaves two blocked goroutines per file, each holding that file's bytes,
  before anything drains them. At the documented "tens of thousands of keys"
  that is ~40k goroutines and 100-200MB resident before ingestion starts. Run
  `fetchPath` inside one goroutine and the walk streams properly. ~5 lines.
- `data/account.go` -- `bindingSet` and `calculateChanges` drain `Bindings()`,
  which spawns a goroutine to feed an unbuffered channel from a slice the
  caller could have had directly, twice per already-known account, on the
  single ingestion goroutine. Add `BindingList() []KeyBindingImpl`. Also lets
  `doBindingChannel` in `connection/doconnection.go` go away.
- `data/account.go` -- `mergeBindings` round-trips every binding through JSON
  to deduplicate a struct that is already comparable. `map[KeyBindingImpl]bool`
  plus `sort.Slice` is shorter than what is there and keeps the stable ordering
  the comment promises.
- `history/history.go` -- one unbuffered `write(2)` per event with the mutex
  held. A steady fleet writes nothing, but the *first* fetch of a fleet is all
  transitions: ~40k syscalls serialising the two ingest goroutines against each
  other. Wrap the file in a `bufio.Writer` and flush in `Close`.
- `lib/library.go` `deserialize` unmarshals the whole file into
  `map[string]interface{}` just to read `Type`, then again into the real type;
  `PublicKey.UnmarshalJSON` does it a third time per key. `var probe struct{
  Type string }` -- worth doing when someone touches that function for the
  `Type`-missing panic already filed under Robustness.
- `connection/fileconnection.go` `matches` compiles two regexps for every
  directory entry, and both -- `~$` and `^#.*` -- are `HasSuffix`/`HasPrefix`
  in disguise.

### Re-ratings of items already filed above

- **`lib.sanitize` is not injective** is worse than billed for `changes/`.
  Two accounts whose IDs differ only in punctuation (`alice@a.b.com` /
  `alice@a-b.com` -- an ordinary pair of hostnames) share one file, so `plan`
  writes two pending revocations and one survives. `apply` revokes on one host,
  deletes the change, exits 0, and the other host keeps the key indefinitely,
  every run, forever. The "verify the stored ID on read" half of the fix turns
  it from silent to loud at no migration cost and should be done first. Not
  reachable for `keys/` or `policies/`, which are keyed by fingerprints and
  `AKIA...` IDs.
- **`SshCmd` has no timeout** -- the new `--timeout` flag now advertises a
  guarantee it cannot deliver. `fetchSudo` checks `ctx.Err()` only between
  accounts, so a host that accepts the connection and goes quiet blocks in
  `Run`'s `ReadLine` forever and `fetch` hangs past its deadline. Say so in the
  flag's usage string until the deadline exists. Its blast radius also grew:
  `apply` now sits on `Close` -> `cmd.Wait()` with no deadline.
- **Panics reachable from repository content** now also lands in `apply`.

### Worth one check against a real host

Whether `sed -i` under `sudo` preserves the owner of
`~user/.ssh/authorized_keys` on every sed implementation targeted. If BusyBox
sed does not, `delKey` leaves the file owned by root and sshd's `StrictModes`
locks the user out.

### Below LOW, noted only

`DOAccount.Email` writes the account holder's email address into the shared
repository. `fetchPath` follows symlinks with no cycle guard, so a symlink loop
under a connected directory recurses until the stack is exhausted.

### Untested: output.ErrorCount, and cancellation on four of five connections

Two of the test reviewer's HIGH findings, left open deliberately. Both are test
coverage over machinery that already shipped, not new defects.

**`output.ErrorCount` has no test at all.** `output_test.go` covers `IsLevel`,
level gating and formatting, and nothing else. The counter was rewritten in this
cycle -- it used to gate on `l >= ErrorLevel` with `ErrorLevel` at iota 0, so
every leveled call incremented it and `locksmith` exited 1 on every invocation,
and `ErrorCount()` used to close `errorChannel` so calling it twice panicked.
Both are fixed, and nothing pins either. What to pin: that only `Error*` calls
increment; that a `Debug` call suppressed by the level gate does not; that the
count survives being read twice; and that an output call after a read does not
panic. The first of those is the one that regressed before.

**Only `FileConnection` has cancellation tests.** `TestFileConnectionStopsWhenCancelled`
and `TestFileConnectionCancelledMidwayStillCloses` are the whole of it. Nothing
drives `Fetch(ctx)` cancellation for SSH, AWS, GitHub or DO, nothing drives the
`--timeout` flag, and nothing drives signal handling.

This is the same gap as the "Cancellation is still missing on three
connections" item above, seen from the other side: AWS, GitHub and DO do bare
channel sends *and* have no test that would notice. Fix them together -- a test
that abandons a `Fetch` and asserts the producer goroutines exit is what makes
the `sendKey`/`sendAccount` conversion verifiable rather than hopeful.

Note `--timeout` cannot currently be tested end to end against a hung host,
because `SshCmd.Run` has no deadline (see the re-rating above). A test would
pin the flag's *parsing* and the cancellation plumbing, not the guarantee.
