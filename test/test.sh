#!/bin/bash

# run some basic tests

# set up our mock SSH commands

mydir=`dirname $0`
source $mydir/test-environment
setup $mydir

rm -rf $HOME/.locksmith

for f in $mydir/tests/test*.sh; do
    echo -n "$f..."
    out=$mydir/output/`basename $f .sh`
    expect=$mydir/expected/`basename $f .sh`
    mkdir -p $out
    bash -c $f > $out/output 2>$out/error
    result=$?
    echo $result > $out/status

    if diff -r "$expect" "$out" ; then
	echo SUCCESS
    else
	echo FAILED
	errors=$(( $errors + 1 ))
    fi
done

exit $errors