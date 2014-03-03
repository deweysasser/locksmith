#!/bin/bash

locksmith servers fetch root@server1
locksmith servers fetch root@server2

locksmith keys enroll test/keys/key1.pub
locksmith keys enroll test/keys/key2.pub
locksmith keys enroll test/keys/key3.pub

locksmith servers add-key key1 server1
locksmith servers add-key key2 server2

locksmith servers update

locksmith servers with-key key1
locksmith servers without-key key1

locksmith keys expire key1
locksmith servers add-key key3

locksmith servers status

locksmith servers update
