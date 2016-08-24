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
	"github.com/chrisliu529/gopl.io/ch6/intset"
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
	lc := levelConfigs[level - 1]
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
	copy(slice[i:], slice[i + 1:])
	return slice[:len(slice) - 1]
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
		if len(mines) > 1 {
			//ignore empty lines
			board := initBoard(toInt(mines))
			fmt.Println(board)
			board.dump(fmt.Sprintf("%d.png", i))
			player := initPlayer(board)
			player.play(board)
		}
	}
}

type TileInt int
type Board struct {
	tiles  [][]TileInt
	level  int
	row    int
	col    int
	mine   int
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
	r, c := b.row, b.col
	tiles := []*TileInt{}
	if x + 1 < c {
		tiles = append(tiles, &b.tiles[y][x + 1])
	}
	if x - 1 >= 0 {
		tiles = append(tiles, &b.tiles[y][x - 1])
	}
	if y + 1 < r {
		tiles = append(tiles, &b.tiles[y + 1][x])
		if x + 1 < c {
			tiles = append(tiles, &b.tiles[y + 1][x + 1])
		}
		if x - 1 >= 0 {
			tiles = append(tiles, &b.tiles[y + 1][x - 1])
		}
	}
	if y - 1 >= 0 {
		tiles = append(tiles, &b.tiles[y - 1][x])
		if x + 1 < c {
			tiles = append(tiles, &b.tiles[y - 1][x + 1])
		}
		if x - 1 >= 0 {
			tiles = append(tiles, &b.tiles[y - 1][x - 1])
		}
	}
	return tiles
}

func (b *Board) setMine(mine int) {
	x, y := toXY(mine, len(b.tiles))
	b.tiles[y][x] = -1
}

func toXY(index, column int) (int, int) {
	return index % column, index / column
}

func toIndex(x, y, column int) int {
	return y * column + x
}

func (b *Board) initTiles() {
	c := levelConfigs[b.level - 1]
	b.row, b.col, b.mine = c.row, c.column, c.mine
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
	width, height := 16 * b.col, 16 * b.row
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range b.tiles {
		for x, v := range b.tiles[y] {
			imgFile := "bomb_gray.png"
			if v >= 0 {
				imgFile = fmt.Sprintf("%d.png", v)
			}
			render(img, 16 * x, 16 * y, imgFile)
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
			img.Set(x + i, y + j, srcImg.At(i, j))
		}
	}
}

func (t *TileInt) isMine() bool {
	return *t == -1
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
	value    int
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
	view  map[*intset.IntSet]int
	sure  int
	guess int
	row   int
	col   int
	mine  int
}

func initPlayer(b *Board) *Player {
	fmt.Println("init player with level ", b.level)
	p := &Player{sure: 0, guess: 0}
	p.init(b)
	return p
}

func (p *Player) init(b *Board) {
	p.row, p.col, p.mine = b.row, b.col, b.mine
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
	step := 0
	click_f := func(x, y int) {
		fmt.Println("try click0 (%d, %d)=%s", x, y, p.tiles[y][x])
		if p.tiles[y][x].value == Unknown {
			p.sure++
			p.click(b, x, y)
			p.dump(fmt.Sprintf("p%d.png", step))
			step ++
		}
	}
	var prev_safe []int
	for b.status == 0 {
		if p.sure == 0 {
			//always click the middle of board for the first step
			click_f(5, 5)
		} else {
			safe, err := p.findSafe()
			check(err)
			fmt.Println("found safe:", safe, prev_safe)
			if compareSlice(prev_safe, safe) {
				fmt.Println("safe fronze: %s, exiting", safe)
				return
			}
			prev_safe = make([]int, len(safe))
			copy(prev_safe, safe)
			for _, v := range safe {
				fmt.Printf("try click v=%d\n", v)
				click_f(toXY(v, p.col))
			}
		}
	}
}

func compareSlice(s1, s2 []int) bool {
	if len(s1) != len(s2) {
		return false
	}
	for i, v := range s1 {
		if v != s2[i] {
			return false
		}
	}
	return true
}

func (p *Player) refreshView() error {
	p.view = make(map[*intset.IntSet]int)
	for y := range p.tiles {
		for x, t := range p.tiles[y] {
			if t.revealed {
				v := t.value
				if v > 0 && v <= 8 {
					s, v1 := p.circle(x, y, v)
					if (v1 < 0) {
						return fmt.Errorf("bad value: (%d, %d) = %d", x, y, v1)
					}
					if (s.Len() > 0) {
						p.view[s] = v1
					}
				}
			}
		}
	}
	return nil
}

func (p *Player) circle(x, y, v int) (*intset.IntSet, int) {
	var s intset.IntSet
	r, c := p.row, p.col

	var f = func(xt, yt int) {
		vt := p.tiles[yt][xt].value
		if (vt == Unknown) {
			s.Add(toIndex(xt, yt, c))
		} else if (vt == Flag) {
			v --
		}
	}
	if x + 1 < c {
		f(x + 1, y)
	}
	if x - 1 >= 0 {
		f(x - 1, y)
	}
	if y + 1 < r {
		f(x, y + 1)
		if x + 1 < c {
			f(x + 1, y + 1)
		}
		if x - 1 >= 0 {
			f(x - 1, y + 1)
		}
	}
	if y - 1 >= 0 {
		f(x, y - 1)
		if x + 1 < c {
			f(x + 1, y - 1)
		}
		if x - 1 >= 0 {
			f(x - 1, y - 1)
		}
	}
	return &s, v
}

func (p *Player) findSafe() ([]int, error) {
	stateChanged := true
	safe := []int{}
	for stateChanged {
		stateChanged = false
		err := p.refreshView()
		if (err != nil) {
			return safe, err
		}
		for s, v := range p.view {
			if v == s.Len() {
				for _, e := range s.Elems() {
					x, y := toXY(e, p.col)
					if p.tiles[y][x].value != Flag {
						p.tiles[y][x].value = Flag
						stateChanged = true
					}
				}
			} else if (v == 0) {
				for _, e := range s.Elems() {
					safe = append(safe, e)
				}
			}
		}
	}
	if (len(safe) == 0) {
		return safe, fmt.Errorf("ai has to guess")
	}
	return safe, nil
}

func (p *Player) click(b *Board, x, y int) {
	fmt.Printf("try click1 x=%d y=%d\n", x, y)
	if y >= b.row || y < 0 || x < 0 || x >= b.col {
		return
	}
	fmt.Println("try click2", p.tiles[y][x])
	if p.tiles[y][x].revealed {
		return
	}
	fmt.Printf("effective click: (%d, %d)\n", x, y)
	t := b.tiles[y][x]
	p.tiles[y][x].value = int(t)
	p.tiles[y][x].revealed = true
	if t.isMine() {
		p.tiles[y][x].value = Boom
		b.status = Boom
		return
	}

	if t == 0 {
		p.click(b, x - 1, y)
		p.click(b, x + 1, y)
		p.click(b, x, y - 1)
		p.click(b, x, y + 1)
	}
}

func (p *Player) dump(outpath string) {
	file, err := os.Create(outpath)
	check(err)
	defer file.Close()
	width, height := 16 * p.col, 16 * p.row
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range p.tiles {
		for x, t := range p.tiles[y] {
			imgFile := "unknown.png"
			v := t.value
			if v >= 0 && v <= 8 {
				imgFile = fmt.Sprintf("%d.png", v)
			} else if v == Boom {
				imgFile = "bomb_red.png"
			} else if v == Flag {
				imgFile = "flag.png"
			}
			render(img, 16 * x, 16 * y, imgFile)
		}
	}
	png.Encode(file, img)
}
