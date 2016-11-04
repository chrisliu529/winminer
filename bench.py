#!/usr/bin/python

import subprocess
import sys
import re
import time

def get_output(cmd):
    p = subprocess.Popen(cmd, stdout=subprocess.PIPE, shell=True)
    out, err = p.communicate()
    if p.returncode != 0:
        print out
        print err
        sys.exit(p.returncode)
    return out.rstrip() #remove '\n' in the end

def wins(s):
    return [int(w) for w in re.match(r'.*\((.*)\).*', s).group(1).split(',')]

def score(w):
    return w[0] + 2*w[1] + 4*w[2]

def ratio(w):
    return [('%s%%' % int(round(f*100))) for f in [w[0]/10000.0, w[1]/5000.0, w[2]/2500.0]]

def bench():
    t = time.time()
    out = get_output("./winminer | grep win:")
    print 'Finish benchmark in %.2f seconds' % (time.time() - t)
    ws = wins(out)
    si = score(ws)
    print 'score=%s %s %s' % (si, ws, ratio(ws))
    if len(sys.argv) == 1:
        return
    if si > int(sys.argv[1]):
        sys.exit(0)
    sys.exit(1)

bench()
