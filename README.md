locksmith
=========

Current Status:  Early Alpha
----------------------------

This project is early Alpha quality only.

It defnitely has bugs.  The storage format will definitely change in
incompatible ways.  It may not work at all.

All that being said, it's being released now to get feedback on what
people want it to do.  Right now it's a useful survey tool to discover
keys and where they're used.  Please give feedback in the form of
Github issues.

Overview
--------

A tool for managing SSH keys on remote servers.

* Ingest SSH and AWS keys from disk, remote SSH servers, Amazon AWS,
  Github, Gitlab
* Delete and Rotate keys, including handling previously expired key in
  newly encountered systems.
* Handle key accessiblity mechanism including bastion hosts/jump
  boxes, sudo and hosts ony reachable within certain networks
* Works well at large and small scale

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

There are 2 branches in this code:

* master -- the 'release' branch (such as it is)

* development -- active development that will be eventually merged to
  the master branch

* prototype -- a BASH prototype of this system used to explore some
  basic features and user interactions and to prove the conept that
  this is useful (it is!  Very!).  The prototype software is slow and
  has only worked on cygwin and Linux.  It is currently the only
  branch that supports key rotation.  It does not support AWS keys.
  
If you would like to try out the prototype, a much more extensive
Readme is availabe on the "prototype" branch at
https://github.com/dmsasser/locksmith/tree/prototype


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
* Remove or replace SSH keys on remote systems
* Ingest SSH keys for other accounts via a sudo capable account
* Manage SSH keys in AWS
* Manage SSH keys in Gitlab and Github
* Manage SSH keys in GCP
* Mnaage SSH keys in Digital Ocean
* Manage Gitlab & Github SSH deployment keys
* Manage SSH host keys
