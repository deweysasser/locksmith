locksmith
=========

A tool for cataloging and managing SSH and AWS keys on local and
remote systems.

Quick Start
-----------

```
locksmith connect -sudo ubuntu@somewhere.example.com
locksmith connect somebody@somewhereelse.example.com
locksmtih connect aws:default
locksmith connect ~/.ssh
locksmith fetch
locksmith list
```

Run `locksmith` for details on subcommands and options.

See the ["Using Locksmith"](#using_locksmith) section below for more
details on operation.

Current Status:  Becoming useful
--------------------------------

This project is Alpha quality only.

It defnitely has bugs.  The storage format will definitely change in
incompatible ways.  It may not work at all.

All that being said, it's being released now to get feedback on what
people want it to do.  Right now it's a useful survey tool to discover
keys and where they're used and has proto features to manipulate SSH
keys on remote systems.

It is already sufficiently performant enough to apply to hundreds of
connections and thousands of keys.  (Thousands of systems and 10s of
thousnands of keys are likely also possible but not well exercised.)

Please give feedback in the form of Github issues.

Current Functionality
---------------------

* Ingest SSH *Public* keys and AWS keys *IDs* from disk, remote SSH
  servers, Amazon AWS (does not store private keys)
* Filter and display keys and key ages.  Mark keys for deletion or replacement
* Handle use `sudo` to access all accounts on a given system through a
  single connection
* Suitable for use with a few keys only
* Highly performant at large scale

Vision
------

A tool for managing SSH keys on remote servers.

* Also ingest/manage keys in Github, Gitlab and other systems
* Delete and Rotate keys, including handling previously expired key in
  newly encountered systems.
* Modify local configuration files (e.g. ~/.aws/credentials) with up
  to date keys
* Handle key accessiblity mechanism including bastion hosts/jump
  boxes, sudo and hosts ony reachable within certain networks

Target Users
------------

The tool is targeted at several use cases:

* An individual user who wants to manage their own SSH keys on
  multiple systems
  
* An operations engineer who needs to manage sets of SSH keys across
  thousands of systems
  
Getting the tool
----------------

This code is *NOT* yet ready for reliable production use.  

### Binaries

Pre-built binaries for 64 bit Windows, Linux and MacOS can be found on
the GitHub Release page.

### Source

As with the current state of golang technology, the `master` branch is
the release branch.  That means it should be (relatively) workable.
Active development is happening on the `development` branch.

There are 3 branches in this code:

* master -- the 'release' branch (such as it is)

* development -- active development that will be eventually merged to
  the master branch

* prototype -- a BASH prototype of this system used to explore some
  basic features.  This is now only of historical interest.
  
Data Storage
------------

Data is stored in `~/.x-locksmith`.  It will eventually move to
`~/.locksmith` when the data format is stable.

SSH Private keys and AWS secret key ids are *NOT* stored so the
repository is suitable for sharing amongst e.g. an operations team via
GIT.  Repository objects are stored in individual JSON files and
should be fairly safe to have GIT merge -- in case on conflicts you
can manually resolve or just allow one side of the merge to win (and
then `fetch` updated data).

Performance
-----------

Current rough performance:

* ingest 10000 SSH keys from disk in 208 seconds on 2-core,
  hyperthreaded 2.8GHz system with 16G of RAM (fully consumes CPU,
  memory usage not apparant at system level).
  
* Fetch from accounts in parallel:

```
time locksmith fetch
Discovered 68 keys in 123 locations
Discovered 122 accounts in 224 references
Fetched from 8 connections
real    0m4.401s
user    0m0.000s
sys     0m0.015s
```

Using Locksmith
---------------

### Connections

It all starts with a connection.  A connection is *how* to connect to
a given system to fetch keys and other information.  There are
currently 3 kinds:

* files/directories (indicated by path)
* SSH remote hosts (assumed when a path does not exists)
* AWS accounts (indicated by "aws:" prefix)

Adding a connection does *NOT* immediately use it -- for that you must
use `locksmith fetch`


#### "expanded" connections

Certain types of connections can expand to reach not only the directly
connected endpoint but other endpoints reachable from it.

For example, if an ssh connection is added with the `--sudo` flag,
then fetch looks up all login accounts on the remote system and uses
passwordless sudo to fetch keys for all accounts.

Setting the "sudo" flag is the default when the login name is `ubuntu`
or `ec2-user`.

Note that there is no current provision for handling passwords with
sudo.

### Fetching information

Once you've added 1 or more connecitons, run `locksmith fetch` to
connect to the systems and fetch information.  As much as possible,
errors are reported and processing continues.

### Displaying keys/systems/etc

Use `locksmith list` to show information.  Information of *all* types
is displayed.  Adding the `-v` flag shows additional details
(e.g. shows the keys on an account or the accounts using a key).

There is a *lot* of information, which brings us to...

### Filtering the commands

All commands take an arbitrary set of string arguments which are used
as filters and only run on objects matching the filters.

Each argument is taken as a separate filter and a filter matches if
the word is a substring of *any part* of the *displayed* line for the
information.  Using "verbose" (the `-v` flag) shows additional
information for objects but does *NOT* impact which objects match the
filter.

If multiple arguments are specified, the set of operands are the
*union* of the filters.

For example, to show only keys you might use

```locksmith list key```

To show only SSH keys you would use

```locksmith list SSHKey```

To show only connections use

```locksmith list connection```

Filters apply to all operations the same, so you can e.g. apply the
`fetch` command to only certain machines using a filter:

```locksmith fetch exmple```

Will only fetch from connections with the string 'example' in them
(which might include "aws:example" and "me@login.example.com".

### Removing objects from locksmith

The `remove` subcommand removes the objects (matching the filter) from
locksmith's knowledge.  This does *NOT* remove a key from the remote
system.  (NOTE:  suggestions on a better name for this subcommand are
welcome).

If you remove a key and then fetch from a connection that got you the
key, the key will come back.

### Adding, removing, Expiring, and manipulating keys

`locksmith add` is used mark a key to be added to a connection

`locksmith expire` is used to mark a key as expired (which implies
that it should be removed from a system)

`locksmith plan` will calculate a set of changes to be applied to
systems (use `locksmith list change` to view them)

`locksmith apply` will apply the pending changes to the various
systems.

Note that expired keys are *NOT* forgotten -- they are kept around so
they can be recognized in other places.  It's possible to expire a
key, then find it in use somewhere new, then have locksmith remove the
key from that new location.


### Additional rare commands

`add-id` can be used to give an additional ID to a key.  Key IDs are,
in general, automatically calculated, but it is not possible to
calculate the AWS fingerprint of an SSH key generated by AWS without
the private key.

`display-lib` displays the contents of the library -- this is really
only useful for debugging.

Road Map
--------

This road map applies to the development branch.  The prototype branch
implements most of the SSH key & accont handling functions is no
longer under development.

* (DONE) Ingest SSH keys from files and remote SSH systems
* (DONE) Use AWS Key fingerprints from AWS and report instances using
  them as root key.  (CAVEAT:  AWS SSH fingerprints are privately
  invented and we do not yet handle them, but will)
* (DONE) Ingest AWS keys from files and AWS
* (DONE) Report on which keys are found on which systems
* (DONE) Refresh state of all known systems.  (CAVEAT:  we do not yet
  handle auto-removing AWS instances which have been terminated)
* (DONE) Ingest SSH keys for other accounts via a sudo capable account
* Remove or replace SSH keys on remote systems
* Manage SSH keys in AWS
* Manage SSH keys in Gitlab and Github
* Manage SSH keys in GCP
* Mnaage SSH keys in Digital Ocean
* Manage Gitlab & Github SSH deployment keys
* Manage SSH host keys
* Edit ~/.aws/credentials file to update as keys rotate
* Fetch keys from aws and create credentials file

"Blue Sky" ideas
----------------

* Interact with hashcorp's Vault?
* use shared storage such as consul
