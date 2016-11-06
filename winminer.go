package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"github.com/chrisliu529/gopl.io/ch6/intset"
	"image"
	"image/png"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sort"
)

type LevelConfig struct {
	row    int
	column int
	mine   int
}

var (
	levelConfigs = []LevelConfig{
		{9, 9, 10},
		{16, 16, 40},
		{16, 30, 99},
	}
	images map[string]image.Image
	dumpPng int
)

func main() {
	gb := flag.Bool("gb", false, "generate benchmark cases or not")
	level := flag.Int("lv", 1, "game level (1-3)")
	n := flag.Int("n", 1, "number of benchmark cases")
	s := flag.Int("s", 0, "random seed for generating benchmark cases")
	f := flag.String("f", "cases.txt", "input file of benchmark cases")
	d := flag.Int("d", 0, "dump board into .png file - 0: don't dump; 1: only dump worthy failures; 2: dump all")

	flag.Parse()

	dumpPng = *d
	if *gb {
		genBench(*n, *s, *level)
	} else {
		if dumpPng > 0 {
			images = loadImages()
		}
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
		bc := benchCase(level, rng)
		sort.Ints(bc)
		printBenchCase(bc)
	}
}

func printBenchCase(bc []int) {
	for i := range bc {
		if i > 0 {
			fmt.Print(",")
		}
		fmt.Printf("%d", bc[i])
	}
	fmt.Print("\n")
}

func benchCase(level int, rng *rand.Rand) []int {
	lc := levelConfigs[level - 1]
	row := lc.row
	col := lc.column
	tiles := row * col
	mine := lc.mine
	res := make([]int, mine)
	mineCandidates := make([]int, tiles)
	for i := 0; i < tiles; i++ {
		mineCandidates[i] = i
	}
	initClick := toIndex(INIT_X, INIT_Y, col)
	for i := 0; i < mine; {
		mineTile := rng.Intn(len(mineCandidates))
		if mineCandidates[mineTile] == initClick {
			/* According to winmine game implementation, the board will be re-shuffled when first click on a mine.
			 * We simply fix the first click by avoiding putting a mine on it.
			 */
			continue
		}
		res[i] = mineCandidates[mineTile]
		i++
		mineCandidates = remove(mineCandidates, mineTile)
	}
	return res
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
	var total int
	var sure, guess, total_clicks int
	wins := make([]int, len(levelConfigs))
	loses := make([]int, len(levelConfigs))
	text, err := ioutil.ReadFile(filename)
	check(err)
	lines := strings.Split(string(text), "\n")
	for i := range lines {
		mines := strings.Split(lines[i], ",")
		if len(mines) > 1 {
			//ignore empty lines
			total++
			board := initBoard(toInt(mines))
			fmt.Println(board)
			if dumpPng == 2 {
				board.dump(fmt.Sprintf("%d.png", i))
			}
			player := initPlayer(board, i)
			res := player.play(board)
			if res == Win {
				wins[board.level - 1]++
			} else {
				if player.worthDump() {
					player.dump(fmt.Sprintf("f%s.png", player.gamename))
				}
				loses[board.level - 1]++
			}
			sure += player.sure
			guess += player.guess
			total_clicks += (player.sure + player.guess)
		}
	}
	win := sumSlice(wins)
	lose := sumSlice(loses)
	fmt.Printf("win: %.2f (%d, %d, %d), lose: %.2f\n",
		float64(win) / float64(total), wins[0], wins[1], wins[2],
		float64(lose) / float64(total))
	fmt.Printf("sure: %d(%.2f), guess: %d(%.2f)\n",
		sure,
		float64(sure) / float64(total_clicks),
		guess,
		float64(guess) / float64(total_clicks))
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

const (
	INIT_X = 5
	INIT_Y = 5
)

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
		x, y := toXY(mine, b.col)
		b.tiles[y][x] = -1
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
			return i + 1, err
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
	Lose
)

type Player struct {
	tiles    [][]TileExt
	view     map[*intset.IntSet]int
	sure     int
	guess    int
	row      int
	col      int
	mine     int
	gamename string
}

func initPlayer(b *Board, i int) *Player {
	fmt.Println("init player with level ", b.level)
	p := &Player{sure: 0, guess: 0, gamename: fmt.Sprintf("%d-%d", b.level, i)}
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

func (p *Player) play(b *Board) int {
	step := 0
	click_f := func(x, y int) {
		if p.tiles[y][x].value == Unknown {
			p.sure++
			p.click(b, x, y)
			step++
		}
	}
	for b.status == 0 {
		if p.sure == 0 {
			//always click the middle of board for the first step
			click_f(INIT_X, INIT_Y)
			continue
		}
		safe := p.findSafe()
		if p.mine == 0 {
			b.status = Win
			break
		}
		if len(safe) == 0 {
			fmt.Println("now we have to guess...")
			x, y := p.doGuess()
			fmt.Printf("guess at (%d, %d)\n", x, y)
			p.guess++
			p.click(b, x, y)
			step++
			continue
		}
		for _, v := range safe {
			click_f(toXY(v, p.col))
		}
	}
	if b.status == Win {
		fmt.Println("Win!")
		return Win
	}
	fmt.Println("Lost!")
	return Lose
}

func inSlice(i int, s []int) bool {
	for _, e := range s {
		if i == e {
			return true
		}
	}
	return false
}

func sumSlice(s []int) int {
	res := 0
	for _, e := range s {
		res += e
	}
	return res
}

func (p *Player) doGuess() (int, int) {
	corners := func() (int, int) {
		if isUnknown(&p.tiles[0][0]) {
			return 0, 0
		}
		if isUnknown(&p.tiles[p.row - 1][p.col - 1]) {
			return p.col - 1, p.row - 1
		}
		if isUnknown(&p.tiles[p.row - 1][0]) {
			return 0, p.row - 1
		}
		if isUnknown(&p.tiles[0][p.col - 1]) {
			return p.col - 1, 0
		}
		return -1, -1
	}
	leftUpper := func() (int, int) {
		return p.one(isUnknown)
	}
	rightBottom := func() (int, int) {
		for y := p.row - 1; y >= 0; y -- {
			for x := p.col - 1; x >= 0; x -- {
				if isUnknown(&p.tiles[y][x]) {
					return x, y
				}
			}
		}
		return -1, -1
	}
	leftBottom := func() (int, int) {
		for y := p.row - 1; y >= 0; y -- {
			for x := 0; x < p.col; x ++ {
				if isUnknown(&p.tiles[y][x]) {
					return x, y
				}
			}
		}
		return -1, -1
	}
	rightUpper := func() (int, int) {
		for y := 0; y < p.row; y ++ {
			for x := p.col - 1; x >= 0; x -- {
				if isUnknown(&p.tiles[y][x]) {
					return x, y
				}
			}
		}
		return -1, -1
	}
	methods := []func() (int, int){leftUpper, rightBottom, leftBottom, rightUpper}
	x, y := corners()
	if x < 0 {
		x, y = methods[p.guess % 4]()
	}
	return x, y
}

func (p *Player) refreshView() error {
	p.view = make(map[*intset.IntSet]int)
	for y := range p.tiles {
		for x, t := range p.tiles[y] {
			if t.revealed {
				v := t.value
				if v > 0 && v <= 8 {
					s, v1 := p.circle(x, y, v)
					if v1 < 0 {
						return fmt.Errorf("bad value: (%d, %d) = %d", x, y, v1)
					}
					if s.Len() > 0 {
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
		if vt == Unknown {
			s.Add(toIndex(xt, yt, c))
		} else if vt == Flag {
			v--
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

func isUnknown(t *TileExt) bool {
	return t.value == Unknown
}

func isNumber(t *TileExt) bool {
	return t.value >= 0 && t.value <= 8
}

func isFlag(t *TileExt) bool {
	return t.value == Flag
}

func (p *Player) findSafe() []int {
	var stateChanged, needRefresh bool
	needRefresh = true
	stateChanged = true
	safe := []int{}
	for stateChanged || needRefresh {
		stateChanged = false
		if needRefresh {
			err := p.refreshView()
			if err != nil {
				check(err)
			}
		}
		needRefresh = false
		for s, v := range p.view {
			if v == s.Len() {
				for _, e := range s.Elems() {
					x, y := toXY(e, p.col)
					fmt.Println(x, y, p.tiles[y][x].value)
					if p.tiles[y][x].value != Flag {
						p.tiles[y][x].value = Flag
						p.mine--
						if p.mine == 0 {
							return p.collect(isUnknown)
						}
						needRefresh = true
					}
				}
			} else if v == 0 {
				for _, e := range s.Elems() {
					if !inSlice(e, safe) {
						safe = append(safe, e)
					}
				}
			}
			if len(safe) > 0 {
				return safe
			}
		}
		if !needRefresh {
			fmt.Println("searching view directly found no safe tiles, start searching diff")
			diff := make(map[*intset.IntSet]int)
			for s, v := range p.view {
				if v > 0 {
					for s2, v2 := range p.view {
						if !s.ProperContains(s2) {
							continue
						}
						d := s.Copy()
						d.DifferenceWith(s2)
						d2 := p.viewKey(d)
						nv := v - v2
						if val, exists := p.view[d2]; exists {
							if val != nv {
								diff[d2] = nv
							}
						} else {
							diff[d2] = nv
						}
					}
				}
			}
			for k, v := range diff {
				stateChanged = true
				p.view[k] = v
			}
		}
		if !stateChanged {
			fmt.Println("searching view diff made no state change, start searching reduce")
			reduce := make(map[*intset.IntSet]int)
			for s, v := range p.view {
				if v > 1 {
					for _, e := range s.Elems() {
						s2 := s.Copy()
						s2.Remove(e)
						reduce[s2] = v - 1
					}
				}
			}
			for s1, v1 := range reduce {
				for s0, v0 := range p.view {
					if s0.ProperContains(s1) && v0 == v1 {
						s2 := s0.Copy()
						s2.DifferenceWith(s1)
						return s2.Elems()
					}
				}
			}
		}
	}

	if len(safe) == 0 && p.mine > 0 {
		//as map iteration is random in golang
		//try shooting for 10 times
		for shoot := 0; shoot < 10; shoot ++ {
			fmt.Printf("#%d try searching by counting down remained %d mines\n", shoot, p.mine)
			safe = p.findReverse()
			if len(safe) > 0 {
				return safe
			}
		}
		if len(safe) == 0 && p.mine < 5 {
			safe = p.findIsle()
		}
	}
	return safe
}

func (p *Player) findReverse() []int {
	safe := []int{}
	us := p.collect(isUnknown)
	visited := make(map[int]bool)
	for _, e := range us {
		visited[e] = false
	}
	m := p.mine
	for s, v := range p.view {
		toReduce := true
		for _, e := range s.Elems() {
			if visited[e] {
				toReduce = false
				break
			}
			visited[e] = true
		}
		if toReduce {
			m -= v
			if m < 0 {
				panic("internal error")
			}
			if m == 0 {
				for k := range visited {
					if !visited[k] {
						safe = append(safe, k)
					}
				}
				return safe
			}
		}
	}
	return safe
}

/*TODO: the O(n) method below looks very ugly...
However, it may be an overkill to add 2 more maps:
1. string->*intset.IntSet
2. string->int
to work around the map keys equal issue
*/
func (p *Player) viewKey(s *intset.IntSet) *intset.IntSet {
	for s2 := range p.view {
		if s.String() == s2.String() {
			fmt.Println("view key found", s2)
			return s2
		}
	}
	return s
}

type IsleContext struct {
	player *Player
	mine   int
	isle   []int
	safe   []int
	mines  []int
}

func combinations(n, m int, f func([]int)) {
	s := make([]int, m)
	last := m - 1
	var rc func(int, int)
	rc = func(i, next int) {
		for j := next; j < n; j++ {
			s[i] = j
			if i == last {
				f(s)
			} else {
				rc(i + 1, j + 1)
			}
		}
		return
	}
	rc(0, 0)
}

func (ic *IsleContext) solve() ([]int, error) {
	solutions := []*intset.IntSet{}
	combinations(len(ic.isle), ic.mine,
		func(s []int) {
			for _, e := range s {
				x, y := toXY(ic.isle[e], ic.player.col)
				ic.player.tiles[y][x].value = Flag
			}
			if ic.player.isConsistent() {
				ic.safe = ic.player.collect(isUnknown)
				var x intset.IntSet
				x.AddAll(ic.safe...)
				solutions = append(solutions, &x)
				ic.mines = make([]int, ic.mine)
				for i, e := range s {
					ic.mines[i] = ic.isle[e]
				}
			}
			for _, e := range s {
				x, y := toXY(ic.isle[e], ic.player.col)
				ic.player.tiles[y][x].value = Unknown
			}
		})
	if len(solutions) == 0 {
		return nil, errors.New("found no solution")
	}
	if len(solutions) == 1 {
		ic.player.mine = 0
		for _, e := range ic.mines {
			x, y := toXY(e, ic.player.col)
			ic.player.tiles[y][x].value = Flag
		}
		return ic.safe, nil
	}
	// If all possible solutions share the same safe tiles, they must be safe
	s0 := solutions[0].Copy()
	for _, s := range solutions {
		s0.IntersectWith(s)
	}
	if s0.Len() > 0 {
		return s0.Elems(), nil
	}
	return nil, errors.New("found multiple solutions but none sure safe")
}

func (p *Player) isConsistent() bool {
	numbers := p.collect(isNumber)
	for _, n := range numbers {
		x, y := toXY(n, p.col)
		nf := p.neighbors(x, y, isFlag)
		if p.tiles[y][x].value != nf {
			fmt.Printf("inconsistency detected (%d, %d) = %d (!=%d)\n", x, y, p.tiles[y][x].value, nf)
			return false
		}
	}
	return true
}

func (p *Player) neighbors(x, y int, filter func(*TileExt) bool) int {
	r, c := p.row, p.col
	n := 0
	var f = func(xt, yt int) {
		if filter(&p.tiles[yt][xt]) {
			n++
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
	return n
}

func (p *Player) findIsle() []int {
	fmt.Printf("remained mines=%d, locating the isle\n", p.mine)
	empty := []int{}
	isle := p.isle()
	us := p.collect(isUnknown)
	if len(us) == len(isle) {
		fmt.Printf("isle located: %v.\n", isle)
		if len(isle) > 10 {
			fmt.Println("isle too large. giving up")
			return empty
		}
		fmt.Printf("Try mines (%d) simulations\n", p.mine)
		ic := &IsleContext{player: p, mine: p.mine, isle: isle}
		safe, err := ic.solve()
		if err != nil {
			fmt.Println("isle: ", err)
			return empty
		}
		return safe
	}
	return empty
}

func (p *Player) isle() []int {
	x, y := p.one(isUnknown)
	visited := make(map[int]bool)
	result := []int{}
	p.isle0(x, y, &result, visited)
	return result
}

func (p *Player) isle0(x, y int, result *[]int, visited map[int]bool) {
	if y >= p.row || y < 0 || x < 0 || x >= p.col {
		return
	}
	i := toIndex(x, y, p.col)
	if visited[i] {
		return
	}
	visited[i] = true
	if p.tiles[y][x].value == Unknown {
		*result = append(*result, i)
		p.isle0(x - 1, y, result, visited)
		p.isle0(x + 1, y, result, visited)
		p.isle0(x, y - 1, result, visited)
		p.isle0(x, y + 1, result, visited)
	}
}

func (p *Player) one(filter func(*TileExt) bool) (int, int) {
	for y := range p.tiles {
		for x := range p.tiles[y] {
			if filter(&p.tiles[y][x]) {
				return x, y
			}
		}
	}
	return -1, -1
}

func (p *Player) collect(filter func(*TileExt) bool) []int {
	res := []int{}
	for y := range p.tiles {
		for x := range p.tiles[y] {
			if filter(&p.tiles[y][x]) {
				res = append(res, toIndex(x, y, p.col))
			}
		}
	}
	return res
}

func (p *Player) click(b *Board, x, y int) {
	if y >= b.row || y < 0 || x < 0 || x >= b.col {
		return
	}
	if p.tiles[y][x].revealed {
		return
	}
	fmt.Printf("click at (%d, %d)\n", x, y)
	t := b.tiles[y][x]
	p.tiles[y][x].value = int(t)
	p.tiles[y][x].revealed = true
	if t.isMine() {
		p.tiles[y][x].value = Boom
		b.status = Boom
		fmt.Printf("boom at (%d, %d)\n", x, y)
		return
	}

	if t == 0 {
		p.click(b, x - 1, y)
		p.click(b, x - 1, y - 1)
		p.click(b, x + 1, y)
		p.click(b, x + 1, y + 1)
		p.click(b, x, y - 1)
		p.click(b, x + 1, y - 1)
		p.click(b, x, y + 1)
		p.click(b, x - 1, y + 1)
	}
}

func (p *Player) worthDump() bool {
	if dumpPng == 0 {
		return false
	}
	if dumpPng == 2 {
		return true
	}
	return p.sure >= 10
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
