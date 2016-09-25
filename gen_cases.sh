#!/bin/bash
./winminer -gb -lv 1 -s 1 -n 100 > cases.txt
./winminer -gb -lv 2 -s 2 -n 50 >> cases.txt
./winminer -gb -lv 3 -s 3 -n 25 >> cases.txt
