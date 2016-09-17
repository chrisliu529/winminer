#!/usr/bin/python

import subprocess
import sys

def get_output(cmd):
    p = subprocess.Popen(cmd, stdout=subprocess.PIPE, shell=True)
    out, err = p.communicate()
    if p.returncode != 0:
        print out
        print err
        sys.exit(p.returncode)
    return out.rstrip() #remove '\n' in the end

def win_ratio(s):
    return float(s.split(',')[0].split( )[1][:-1])

def bench():
    r = win_ratio(get_output("./winminer | grep win:"))
    print 'win_ratio=%s' % r
    if len(sys.argv) == 1:
        return
    if r > float(sys.argv[1]):
        sys.exit(0)
    sys.exit(1)

bench()
