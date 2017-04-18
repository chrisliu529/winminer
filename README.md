# winminer

[ ![Codeship Status for chrisliu529/winminer](https://codeship.com/projects/d583a910-deff-0133-8b3b-12efcaf3d9f4/status?branch=master)](https://codeship.com/projects/144898)

A small console tool to assist winmine game in windows XP in C code. It reads the layout of field by reading pixels and analyzes the bombs location.

The original version is in winxp branch and must be complied with mingw under Windows system. Due to the changes in winmine game UI, this version ONLY works on XP.

A new version (master branch) is a rewrite in golang to focus on the AI improvements.

## Benchmark

| Level | Beginner | Intermediate | Expert |
|-------|:--------:|:--------------:|:--------:|
| Win   |    86%   |      69%     |   25%  |

### How to Run Benchmark

./bench.sh

It would take a few minutes.

## Search Strategies

### Direct Search

![Direct Search](image/direct.png?raw=true)

Obviously tile A (2, 0) is safe because there is one and only one bomb around (1, 1) and (2, 1) has already been flagged there is one and only one bomb around (1, 2).

This strategy is built-in and non-configurable.

### Diff Search

![Diff Search](image/diff.png?raw=true)

According to (1, 1)=1, there is one and only one bomb among "ABC", noted as "ABC"=1.

According to (1, 0)=1, there is one and only one bomb among "AB", noted as "AB"=1. 

In one hand, "ABC"-"AB"="C"; in another hand, "ABC"-"AB"=1-1=0.

Therefore, "C"=0, tile C is safe.

### Reduce Search

![Reduce Search](image/reduce.png?raw=true)

Let's introduce function L(S) to show there's *at least* L(S) bombs in any given tiles set S.

According to "BCD"=2, we can obtain L("BC")=1.

Combined with "ABC"=1, we can obtain "A"=0, tile A is safe.
