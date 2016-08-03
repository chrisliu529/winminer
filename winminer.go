package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"strconv"
	"strings"
)

type LevelConfig struct {
	row    int
	column int
	mine   int
}

var (
	levelConfigs = []LevelConfig{
		{9, 9, 10},
	}
)

func main() {
	gb := flag.Bool("gb", false, "generate benchmark cases or not")
	n := flag.Int("n", 1, "number of benchmark cases")
	s := flag.Int("s", 0, "random seed for generating benchmark cases")
	f := flag.String("f", "cases.txt", "input file of benchmark cases")

	flag.Parse()

	if *gb {
		level := flag.Int("lv", 1, "game level (1-3)")
		genBench(*n, *s, *level)
	} else {
		runBench(*f)
	}
}

func genBench(n, s, level int) {
	rng := rand.New(rand.NewSource(int64(s)))
	for i := 0; i < n; i++ {
		genBenchCase(level, rng)
	}
}

func genBenchCase(level int, rng *rand.Rand) {
	lc := levelConfigs[level-1]
	row := lc.row
	col := lc.column
	tiles := row * col
	mine := lc.mine
	mineCandidates := make([]int, tiles)
	for i := 0; i < tiles; i++ {
		mineCandidates[i] = i
	}
	for i := 0; i < mine; i++ {
		mineTile := rng.Intn(len(mineCandidates))
		if i == 0 {
			fmt.Printf("%d", mineTile)
		} else {
			fmt.Printf(",%d", mineTile)
		}
		remove(mineCandidates, mineTile)
	}
	fmt.Print("\n")
}

func remove(slice []int, i int) []int {
	copy(slice[i:], slice[i+1:])
	return slice[:len(slice)-1]
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func runBench(filename string) {
	text, err := ioutil.ReadFile(filename)
	check(err)
	lines := strings.Split(string(text), "\n")
	for i := range lines {
		mines := strings.Split(lines[i], ",")
		if len(mines) > 1 { //ignore empty lines
			board := initBoard(toInt(mines))
			fmt.Println(board)
			player := initPlayer(board.level)
			player.play(board)
		}
	}
}

type TileInt int
type Board struct {
	tiles [][]TileInt
	level int
}

func initBoard(mines []int) *Board {
	fmt.Println("init board with ", mines, len(mines))
	level, err := getLevel(len(mines))
	check(err)
	b := &Board{level: level}
	b.setMines(mines)
	return b
}

func toInt(ss []string) []int {
	res := make([]int, len(ss))
	for idx, s := range ss {
		i, err := strconv.Atoi(s)
		check(err)
		res[idx] = i
	}
	return res
}

func (b *Board) setMines(mines []int) {
	b.initTiles()
	for _, mine := range mines {
		b.setMine(mine)
	}
	b.setHints()
}

func (b *Board) setHints() {
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
}

func (b *Board) getNeighbors(x, y int) []*TileInt {
	r, c := len(b.tiles), len(b.tiles[0])
	tiles := []*TileInt{}
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

func (b *Board) setMine(mine int) {
	x, y := toXY(mine, len(b.tiles))
	b.tiles[y][x] = -1
}

func toXY(mine, column int) (int, int) {
	return mine % column, mine / column
}

func (b *Board) initTiles() {
	row, col := getDims(b.level)
	b.tiles = make([][]TileInt, row)
	for i := range b.tiles {
		t := make([]TileInt, col)
		for j := range t {
			t[j] = 0
		}
		b.tiles[i] = t
	}
}

func (b *Board) String() string {
	var buf bytes.Buffer
	for y := range b.tiles {
		fmt.Println(&buf, b.tiles[y])
	}
	return buf.String()
}

func (t *TileInt) isMine() bool {
	return *t == -1
}

func getDims(level int) (int, int) {
	c := levelConfigs[level-1]
	return c.row, c.column
}

func getLevel(n int) (level int, err error) {
	for i := 0; i < len(levelConfigs); i++ {
		if n == levelConfigs[i].mine {
			return i + 1, nil
		}
	}
	return level, errors.New("Level not found")
}

type TileExt int
type Player struct {
	tiles [][]TileExt
	sure  int
	guess int
}

func initPlayer(level int) *Player {
	fmt.Println("init player with level ", level)
	return &Player{}
}

func (p *Player) play(b *Board) {
	fmt.Println("playing")
}
