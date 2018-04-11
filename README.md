locksmith
=========

A tool for managing SSH keys on remote servers.

* Ingest keys from disk, remote SSH servers, Amazon AWS, Github,
  Gitlab
* Rotate and expire keys, including handling previously expired key in
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
  

Getting the code
----------------

This code is *NOT* yet ready for reliable production use.

There are 2 branches in this code:

* prototype -- a BASH prototype of this system used to explore some
  basic features and user interactions and to prove the conept that
  this is useful (it is!  Very!)
  
* development -- a GOLANG implementation in progress that will turn
  into the released tool

If you would like to try out the prototype, a much more extensive
Readme is availabe on the "prototype" branch at
https://github.com/dmsasser/locksmith/tree/prototype

When we reach a minimum useful increment of functionality with data
store which we can expect to preserve going forward, we will start
producing pre-built binaries for the major OSes.


Current Status
--------------

The prototype is working but somewhat slow, dependent on BASH and all
the other programs it calls.

The development version requires building in a Golang environment and
currently only ingests SSH keys from files and remote hosts.  It is
very much in flux and almost certainly has bugs and will change
storage formats as it goes.

Road Map
--------

This road map applies to the development branch.  The prototype branch
implements most of the SSH key & accont handling functions is no
longer under development.

* (DONE) Ingest SSH keys from files and remote systems
* Report on which keys are found on which systems
* Remove or replace SSH keys on remote systems
* Refresh state of all known systems
* Ingest SSH keys for other accounts via a sudo capable account
* Manage SSH keys in AWS
* Manage SSH keys in Gitlab and Github
* Related existing AWS systems to managed launch keys
* Manage SSH keys in GCP
* Mnaage SSH keys in Digital Ocean
* Manage Gitlab & Github SSH deployment keys
* Manage AWS access keys
* Manage SSH host keys
