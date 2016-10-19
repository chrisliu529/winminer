# winminer

A small console tool to assist winmine game in windows XP in C code. It reads the layout of field by reading pixels and analyzes the bombs location.

The original version is in winxp branch and must be complied with mingw under Windows system. Due to the changes in winmine game UI, this version ONLY works on XP.

A new version (master branch) is a rewrite in golang to focus on the AI improvements.

*Benchmark*

| Level | Beginner | Intermediate | Expert |
|-------|:--------:|:--------------:|:--------:|
| Win   |    86%   |      69%     |   25%  |

[ ![Codeship Status for chrisliu529/winminer](https://codeship.com/projects/d583a910-deff-0133-8b3b-12efcaf3d9f4/status?branch=master)](https://codeship.com/projects/144898)
