#!/usr/bin/python

import subprocess
from multiprocessing import Pool

def wins(summary):
    lines = summary.split('\n')
    for line in lines:
        if line.startswith('won:'):
            return int(line.split()[1])

def level_score(level):
    global win_times
    weights = [1,2,4]
    w = wins(subprocess.check_output(["./mine-" + str(level)]))
    return (w, weights[level-1]*w)

def bench():
    p = Pool(4)
    l = p.map(level_score, range(1, 4))
    total = 0
    win_times = []
    for (w, s) in l:
        total += s
        win_times.append(str(w))
    print str(total), '(' + ','.join(win_times) + ')'

bench()
