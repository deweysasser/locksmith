#!/bin/bash

# Purpose:  a basic smoke test

locksmith servers fetch root@server1
locksmith servers fetch root@server2

locksmith keys enroll test/keys/key1.pub

locksmith servers add-key key1

locksmith servers status

locksmith servers update
