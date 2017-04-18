#!/usr/bin/python

import subprocess
import sys
import re
import time
from string import Template
from optparse import OptionParser

parser = OptionParser()
parser.add_option("-c", "--combinations",
                  action="store_true", dest="bench_combinations", default=False,
                  help="run benchmarks with config combinations")
parser.add_option("-f", "--filters",
                  action="store_true", dest="test_filters", default=False,
                  help="test filters of different strategies")

(options, args) = parser.parse_args()

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
    ct = time.time() - t
    ws = wins(out)
    si = score(ws)
    print 'score=%s %s %s, cost %.2f seconds' % (si, ws, ratio(ws), ct)
    if len(args) < 1:
        return
    if si < int(args[0]):
        sys.exit(1)

def gen_config(s, g):
    t = Template(file('template.toml').read())
    print 'strategies = %s, guess = %s' % (s, g)
    f = open('winminer.toml', 'w')
    f.write(t.substitute(strategies=s, guess=g))
    f.close()

def bench_combinations():
    strategies=["diff", "reduce", "isle"]
    gs=["first", "random", "corner"]
    for i in range(len(strategies)):
        for j in range(len(gs)):
            gen_config(strategies[:i+1], gs[j])
            bench()

def test_filter(name, infile):
    get_output("./winminer -dt -dt-file %s -f %s" % (name, infile))

def test_filters():
    infile = 'cases0.txt'
    get_output("./winminer -gb -lv 2 -s 1 -n 100 > %s" % infile)
    strategies=["diff", "reduce"]
    for i in range(len(strategies)):
        st = strategies[:i+1]
        gen_config(st, "corner")
        name = '_'.join(st)
        test_filter(name, infile)
        inflie = name + '_failed.txt'

if options.bench_combinations:
    bench_combinations()
elif options.test_filters:
    test_filters()
else:
    bench()
