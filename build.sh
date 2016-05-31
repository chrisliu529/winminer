#!/bin/bash

for level in `seq 1 3`;
do
    make level=${level} || exit $?
done
