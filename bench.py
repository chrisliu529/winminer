#!/usr/bin/python3

import subprocess
import sys
import re
import time
from string import Template
from optparse import OptionParser
from concurrent.futures import ProcessPoolExecutor

parser = OptionParser()
parser.add_option("-c", "--combinations",
                  action="store_true", dest="bench_combinations", default=False,
                  help="run benchmarks with config combinations")

(options, args) = parser.parse_args()

def get_output(cmd):
    p = subprocess.Popen(cmd, stdout=subprocess.PIPE, shell=True)
    out, err = p.communicate()
    if p.returncode != 0:
        print(out)
        print(err)
        sys.exit(p.returncode)
    return out.rstrip() #remove '\n' in the end

def wins(s):
    return [int(w) for w in re.match(r'.*\((.*)\).*', s).group(1).split(',')]

def score(w):
    return w[0] + 2*w[1] + 4*w[2]

def ratio(w):
    return [('%s%%' % int(round(f*100))) for f in [w[0]/10000.0, w[1]/5000.0, w[2]/2500.0]]

def config_file(s, g):
    return f'{"-".join(s)}-{g}.toml'


def bench(args):
    s, g = args
    with open('template.toml') as f:
        t = Template(f.read())
    c = config_file(s, g)
    with open(c, 'w') as f:
        f.write(t.substitute(strategies=s, guess=g))

    t = time.time()
    out = get_output(f'./winminer -c {c} | grep win:')
    ct = time.time() - t
    ws = wins(out.decode('utf-8'))
    si = score(ws)
    print('strategies = %s, guess = %s' % (s, g))
    print('score=%s %s %s, cost %.2f seconds' % (si, ws, ratio(ws), ct))
    if len(args) < 1:
        return
    if si < int(args[0]):
        sys.exit(1)


def bench_combinations():
    strategies=["diff", "reduce", "isle"]
    gs=["first", "random", "corner", "min"]
    args = []
    for i in range(len(strategies)):
        for j in range(len(gs)):
            args.append((strategies[:i+1], gs[j]))

    with ProcessPoolExecutor() as executor:
        executor.map(bench, args)


if options.bench_combinations:
    bench_combinations()
else:
    bench()
