#!/bin/bash
cd image
~/go/bin/fyne bundle uncertain.png > bundled.go
pngs=( 0 1 2 3 4 5 6 7 8 bomb_gray bomb_red flag unknown )
for i in "${pngs[@]}"
do
	~/go/bin/fyne bundle -append $i.png >> bundled.go
done
cp bundled.go ..
cd ..
