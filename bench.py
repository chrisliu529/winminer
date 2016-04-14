#!/usr/bin/python

import subprocess

def wins(summary):
    lines = summary.split('\n')
    for line in lines:
        if line.startswith('won:'):
            return int(line.split()[1])

def bench():
    score = 0
    weights = [1,2,4]
    for level in range(1,4):
        s = weights[level-1]*wins(subprocess.check_output(["./mine-" + str(level)]))
        score += s
    return score

print bench()
