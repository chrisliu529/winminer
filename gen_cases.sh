#!/bin/bash
./winminer -gb -lv 1 -s 1 -n 1000 > cases.txt
./winminer -gb -lv 2 -s 2 -n 500 >> cases.txt
./winminer -gb -lv 3 -s 3 -n 250 >> cases.txt

