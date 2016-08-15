package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
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
	images map[string]image.Image
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
		images = loadImages()
		runBench(*f)
	}
}

func loadImages() map[string]image.Image {
	res := make(map[string]image.Image)
	files := []string{
		"0", "1", "2", "3", "4", "5", "6", "7", "8",
		"bomb_gray", "bomb_red", "flag",
		"unknown", "uncertain",
	}
	for _, f := range files {
		filename := fmt.Sprintf("%s.png", f)
		path := "image/" + filename
		file, err := os.Open(path)
		check(err)
		defer file.Close()
		img, _, err := image.Decode(file)
		check(err)
		res[filename] = img
	}
	return res
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
		if i > 0 {
			fmt.Printf(",")
		}
		fmt.Printf("%d", mineTile)
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
		log.Fatal(e)
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
			board.dump(fmt.Sprintf("%d.png", i))
			player := initPlayer(board)
			player.play(board)
			player.dump(fmt.Sprintf("p%d.png", i))
		}
	}
}

type TileInt int
type Board struct {
	tiles  [][]TileInt
	level  int
	row    int
	col    int
	status int
}

func initBoard(mines []int) *Board {
	fmt.Println("init board with ", mines, len(mines))
	level, err := getLevel(len(mines))
	check(err)
	b := &Board{level: level, status: 0}
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
	b.row, b.col = getDims(b.level)
	b.tiles = make([][]TileInt, b.row)
	for i := range b.tiles {
		t := make([]TileInt, b.col)
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

func (b *Board) dump(outpath string) {
	file, err := os.Create(outpath)
	check(err)
	defer file.Close()
	width, height := 16*b.col, 16*b.row
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range b.tiles {
		for x, v := range b.tiles[y] {
			imgFile := "bomb_gray.png"
			if v >= 0 {
				imgFile = fmt.Sprintf("%d.png", v)
			}
			render(img, 16*x, 16*y, imgFile)
		}
	}
	png.Encode(file, img)
}

func render(img *image.RGBA, x int, y int, filename string) {
	srcImg := images[filename]
	if srcImg == nil {
		log.Fatal(fmt.Sprintf("image %s not loaded", filename))
	}
	for i := 0; i < 16; i++ {
		for j := 0; j < 16; j++ {
			img.Set(x+i, y+j, srcImg.At(i, j))
		}
	}
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

type TileExt struct {
	value    TileInt
	revealed bool
}

const (
	Unknown = 10 + iota
	Boom    //mine clicked, game over
	Flag    //mine marked
	Win
)

type Player struct {
	tiles [][]TileExt
	sure  int
	guess int
	row   int
	col   int
}

func initPlayer(b *Board) *Player {
	fmt.Println("init player with level ", b.level)
	p := &Player{sure: 0, guess: 0}
	p.initTiles(b)
	return p
}

func (p *Player) initTiles(b *Board) {
	p.row, p.col = b.row, b.col
	p.tiles = make([][]TileExt, p.row)
	for i := range p.tiles {
		t := make([]TileExt, p.col)
		for j := range t {
			t[j].value = Unknown
			t[j].revealed = false
		}
		p.tiles[i] = t
	}
}

func (p *Player) play(b *Board) {
	fmt.Println("playing by random clicks")
	rng := rand.New(rand.NewSource(int64(123)))
	for b.status == 0 {
		x, y := p.check(b, rng)
		p.click(b, x, y)
	}
}

func (p *Player) check(b *Board, rng *rand.Rand) (int, int) {
	fmt.Println("player is checking board")
	if p.sure == 0 {
		//always click the middle of board for the first step
		p.sure++
		return 5, 5
	}
	return rng.Intn(b.col), rng.Intn(b.row)
}

func (p *Player) click(b *Board, x, y int) {
	if y >= b.row || y < 0 || x < 0 || x >= b.col {
		return
	}
	fmt.Println(y, x)
	if p.tiles[y][x].revealed {
		return
	}
	t := b.tiles[y][x]
	p.tiles[y][x].value = t
	p.tiles[y][x].revealed = true
	if t.isMine() {
		p.tiles[y][x].value = Boom
		b.status = Boom
		return
	}

	if t == 0 {
		p.click(b, x-1, y)
		p.click(b, x+1, y)
		p.click(b, x, y-1)
		p.click(b, x, y+1)
	}
}

func (p *Player) dump(outpath string) {
	file, err := os.Create(outpath)
	check(err)
	defer file.Close()
	width, height := 16*p.col, 16*p.row
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range p.tiles {
		for x, t := range p.tiles[y] {
			imgFile := "unknown.png"
			if t.revealed {
				v := t.value
				if v >= 0 && v <= 8 {
					imgFile = fmt.Sprintf("%d.png", v)
				} else if v == Boom {
					imgFile = "bomb_red.png"
				}
			}
			render(img, 16*x, 16*y, imgFile)
		}
	}
	png.Encode(file, img)
}
