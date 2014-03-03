#!/bin/bash

# run some basic tests

# set up our mock SSH commands

mydir=`dirname $0`
source $mydir/test-environment
setup $mydir

rm -rf $HOME/.locksmith

for f in $mydir/tests/test*.sh; do
    echo -n "$f..."
    testname=`basename $f .sh`
    out=$mydir/output/$testname/output
    expect=$mydir/expected/$testname

    export HOME=$mydir/output/$testname/home
    export MOCKSERVERS=$mydir/output/$testname/mock-servers
    mkdir -p $HOME $out
    bash -c $f > $out/stdout.txt 2>$out/stderr.txt
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