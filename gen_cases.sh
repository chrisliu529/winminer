#!/bin/bash
./winminer -gb -lv 1 -s 1 -n 10000 > cases.txt
./winminer -gb -lv 2 -s 2 -n 5000 >> cases.txt
./winminer -gb -lv 3 -s 3 -n 2500 >> cases.txt

