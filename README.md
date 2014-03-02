locksmith 
=========

A tool for managing SSH public keys on man remote servers.

Overview
========

Locksmith is designed to manage OpenSSH keys on remote servers for
you.  It will track what keys are on what servers, track the date
where it first saw a key, track expired keys, allow you to mark keys
for additions on some (or all) servers and update the servers in bulk.


Status
======

		       THIS IS PROTOTYPE CODE.

It's hot off of my fingers.  I'm using it in 2 environments but I
guarantee there are bugs.

The worst this code can do is zorch your authorized keys files on all
your servers.

That being said, please try it out and report bugs to via
https://github.com/dmsasser/locksmith.

You should consider this a functional, potentially useful PROTOTYPE.
That allows for investigation of workflows, functionality and command
line convenience.  Once it evolves a bit I will re-write it pretty
much entirely.  In the meantime, it's a *prototype* so feedback is not
only welcome, it's encouraged.  I'll take feedback in any area from
functionality to command line syntax to deployment issues.

Note that it's written in BASH.  This is not the final target
language, this was a rapid prototype language.  It will be
re-implemented in Python.  If you'd like to submit patches to the
prototype that's great but be aware that it will all be rewritten
eventually.


Features
========

* Manage SSH public keys across many servers
* Track date of public keys (by remembering when they were first seen)
* Track state of SSH keys on servers and perform minimal updates
* Allow expiration of public keys which will automatically remove them
  from all servers at next update.
* Allow addition of public keys to some or all servers in bulk
* Storge local tracking information in plain text (i.e. SCM friendly)
  formats.  It my intention to make these formats automatically deal
  with merge conflict markers as well.

Future Features
===============

* Define hierarchal groups of servers
* Add/remove keys to servers based on group membership
* access servers in parallel
* intergrate with nmap to find servers

Limitations
===========

* Cannot handle command= prefixed keys (or any kind of line that
  doesn't start with "ssh-" in authorized_keys file.

Help
====

Run "locksmith help", "locksmith help servers" and "locksmith help
keys".

Basic Usage
===========

This is a shell example of using locksmith to manage keys in bulk on 2
servers.  It's as easy to do this on 20 servers as 2

## First we give it some servers and fetch their keys
    $ locksmith servers add root@server1
    $ locksmith servers add root@server2
    $ locksmith servers fetch
    fetching from root@server1
    root@server1:
    
    () rsa ...qWHj8ONgYw== key2
    () rsa ...3WNlX+4dWw== key1
    fetching from root@server2
    root@server2:
    () rsa ...qWHj8ONgYw== key2

## We can list all the keys
    $ locksmith keys
    
    (20140302001203) rsa ...qWHj8ONgYw== key2
    (20140302001105) rsa ...3WNlX+4dWw== key1

## And add a new key to the locksmith, but not (yet) to the servers
    $ locksmith keys enroll test/keys/key3.pub 
    $ locksmith keys
    
    (20140302001719) rsa ...qWHj8ONgYw== key2
    (20140302001718) rsa ...3WNlX+4dWw== key1
    (20140302001725) rsa ...T61oyZhZqw== key3

## Now let's update the servers.  First, what do they have?

    $ locksmith servers show
    root@server1:
    
    () rsa ...qWHj8ONgYw== key2
    () rsa ...3WNlX+4dWw== key1
    root@server2:
    () rsa ...qWHj8ONgYw== key2

## OK, I'd like key3 on server2 and let's expire key1 (which will take it out whever it might be found)

    $ locksmith servers add-key key3 root@server2
    Adding to root@server2:
    () rsa ...T61oyZhZqw== key3
    $ locksmith keys expire key1
    Expiring 
    Expiring EXPIRED! (20140302001718) rsa ...3WNlX+4dWw== key1

## But this hasn't yet updated the servers -- as we can see
    $ locksmith servers status
    root@server1:
    EXPIRED! () rsa ...3WNlX+4dWw== key1
    root@server2:
    keys to add:
    () rsa ...T61oyZhZqw== key3

## So let's do the updates:
    $ locksmith servers update
    root@server1:
    fetching from root@server1
    root@server1:
    
    () rsa ...qWHj8ONgYw== key2
    EXPIRED! () rsa ...3WNlX+4dWw== key1
    Removing 1 keys
    root@server1 updated
    New keys are:
    
    () rsa ...qWHj8ONgYw== key2
    root@server2:
    fetching from root@server2
    root@server2:
    () rsa ...qWHj8ONgYw== key2
    Adding 1 keys
    root@server2 updated
    New keys are:
    () rsa ...qWHj8ONgYw== key2
    () rsa ...T61oyZhZqw== key3

## let's verify that it did the right thing
    $ locksmith servers show
    root@server1:
    
    () rsa ...qWHj8ONgYw== key2
    root@server2:
    () rsa ...qWHj8ONgYw== key2
    () rsa ...T61oyZhZqw== key3
    $