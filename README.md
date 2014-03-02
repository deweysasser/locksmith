locksmith 
=========

A tool for managing SSH keys on remote servers.

Status
======

THIS IS ALPHA CODE.

It's hot off of my fingers.  I'm using it in 2 environments but I
guarantee there are bugs.

The worst this code can do is zorch your authorized keys files on all
your servers.

That being said, please try it out and report bugs to via
https://github.com/dmsasser/locksmith.


Overview
========

Locksmith is designed to manage keys on remote servers for you.  It
will track what keys are on what servers, track the date where it
first saw a key, track expired keys, allow you to mark keys for
additions on some (or all) servers and update the servers in bulk.

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