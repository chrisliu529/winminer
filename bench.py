#!/usr/bin/python

import subprocess
from multiprocessing import Pool

def wins(summary):
    lines = summary.split('\n')
    for line in lines:
        if line.startswith('won:'):
            return int(line.split()[1])

def level_score(level):
    weights = [1,2,4]
    return weights[level-1]*wins(subprocess.check_output(["./mine-" + str(level)]))

def bench():
    p = Pool(4)
    return sum(p.map(level_score, range(1, 4)))

print bench()
