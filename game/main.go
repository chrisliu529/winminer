package main

import (
	"fmt"
	"math/rand"

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/container"
)

type levelConfig struct {
	row  int
	col  int
	mine int
}

type tileInt int

func (t *tileInt) isMine() bool {
	return *t == -1
}

type board struct {
	tiles  [][]tileInt
	lc     levelConfig
	status int
	text   string
}

const (
	initx = 0
	inity = 0
)

var (
	rng          *rand.Rand
	theBoard     *board
	levelConfigs []levelConfig
	resMap       map[string]*fyne.StaticResource
)

func toXY(index, column int) (int, int) {
	return index % column, index / column
}

func remove(slice []int, i int) []int {
	copy(slice[i:], slice[i+1:])
	return slice[:len(slice)-1]
}

func newBoard(level int) *board {
	c := levelConfigs[level-1]
	b := &board{lc: c, status: 0}
	b.tiles = make([][]tileInt, b.lc.row)
	for i := range b.tiles {
		t := make([]tileInt, b.lc.col)
		for j := range t {
			t[j] = 0
		}
		b.tiles[i] = t
	}

	tiles := b.lc.row * b.lc.col
	mineCandidates := make([]int, tiles)
	for i := 0; i < b.lc.mine; i++ {
		mineTile := rng.Intn(len(mineCandidates))
		x, y := toXY(mineTile, b.lc.col)
		b.tiles[y][x] = -1
		mineCandidates = remove(mineCandidates, mineTile)
	}

	for y := range b.tiles {
		for x := range b.tiles[y] {
			if b.tiles[y][x] != 0 {
				continue
			}
			neighbors := b.getNeighbors(x, y)
			for _, t := range neighbors {
				if t.isMine() {
					b.tiles[y][x]++
				}
			}
		}
	}

	return b
}

func (b *board) getNeighbors(x, y int) []*tileInt {
	r, c := b.lc.row, b.lc.col
	tiles := []*tileInt{}
	if x+1 < c {
		tiles = append(tiles, &b.tiles[y][x+1])
	}
	if x-1 >= 0 {
		tiles = append(tiles, &b.tiles[y][x-1])
	}
	if y+1 < r {
		tiles = append(tiles, &b.tiles[y+1][x])
		if x+1 < c {
			tiles = append(tiles, &b.tiles[y+1][x+1])
		}
		if x-1 >= 0 {
			tiles = append(tiles, &b.tiles[y+1][x-1])
		}
	}
	if y-1 >= 0 {
		tiles = append(tiles, &b.tiles[y-1][x])
		if x+1 < c {
			tiles = append(tiles, &b.tiles[y-1][x+1])
		}
		if x-1 >= 0 {
			tiles = append(tiles, &b.tiles[y-1][x-1])
		}
	}
	return tiles
}

func canvasScreen(_ fyne.Window) fyne.CanvasObject {
	b := theBoard
	imgs := []fyne.CanvasObject{}
	for y := range b.tiles {
		for _, v := range b.tiles[y] {
			imgName := "bomb_gray"
			if v >= 0 {
				imgName = fmt.Sprintf("%d", v)
			}
			imgs = append(imgs, canvas.NewImageFromResource(resMap[imgName]))
		}
	}

	return container.NewGridWrap(fyne.NewSize(24, 24), imgs...)
}

func main() {
	resMap = map[string]*fyne.StaticResource{
		"bomb_gray": resourceBombgrayPng,
		"0":         resource0Png,
		"1":         resource1Png,
		"2":         resource2Png,
		"3":         resource3Png,
		"4":         resource4Png,
		"5":         resource5Png,
		"6":         resource6Png,
		"7":         resource7Png,
		"8":         resource8Png,
	}
	rng = rand.New(rand.NewSource(0))
	levelConfigs = []levelConfig{{9, 9, 10}}
	theBoard = newBoard(1)
	a := app.NewWithID("minebot")
	w := a.NewWindow("minebot")

	w.SetMaster()

	content := container.NewMax()
	tutorial := container.NewBorder(nil, nil, nil, nil, content)
	w.SetContent(tutorial)
	w.Resize(fyne.NewSize(260, 100)) //size(845, 400) for (30, 16)
	w.SetFixedSize(true)

	repaint := func() {
		content.Objects = []fyne.CanvasObject{canvasScreen(w)}
		content.Refresh()
	}
	repaint()
	w.Canvas().SetOnTypedKey(func(k *fyne.KeyEvent) {
		if k.Name == fyne.KeySpace {
			repaint()
		}
	})
	w.ShowAndRun()
}
