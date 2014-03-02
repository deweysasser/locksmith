#!/bin/bash

# Purpose:  a basic smoke test

locksmith servers fetch root@server1
locksmith servers fetch root@server1

locksmith keys enroll keys/key1.pub

locksmith servers add-key key1

locksmith servers status

locksmith servers update
