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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/chrisliu529/gopl.io/ch6/intset"
)

type levelConfig struct {
	Row    int
	Column int
	Mine   int
}

type isleConf struct {
	MaxMine int
	MaxSize int
}

type tomlConfig struct {
	Levels     []levelConfig
	Strategies []string
	Guess      string
	Accept     float64
	Verbose    bool
	Isle       isleConf
}

type stats struct {
	success int
	failure int
}

var (
	config       tomlConfig
	images       map[string]image.Image
	dumpPng      int
	rng          *rand.Rand
	dumpText     bool
	successCases *os.File
	failedCases  *os.File
	pngCounter   int
	clickCounter int
)

func main() {
	pngCounter = 1
	cfgFile := flag.String("c", "winminer.toml", "config file name")
	gb := flag.Bool("gb", false, "generate benchmark cases or not")
	n := flag.Int("n", 1, "number of benchmark cases to be generated, only works with -gb")
	s := flag.Int("s", 0, "random seed for generating benchmark cases, only works with -gb")
	level := flag.Int("lv", 1, "game level (1-3) for generating benchmark cases, only works with -gb")
	dt := flag.Bool("dt", false, "dump text of cases or not")
	dtFile := flag.String("dt-file", "", "file name prefix for dumping cases")
	f := flag.String("f", "cases.txt", "input file of benchmark cases")
	d := flag.Int("d", 0, "dump board into .png file - 0: don't dump; 1: only dump worthy failures; 2: dump all; 3: dump all pictures with file names in sequence to make video")
	flag.Parse()

	if _, err := toml.DecodeFile(*cfgFile, &config); err != nil {
		fmt.Println(err)
		return
	}

	rng = rand.New(rand.NewSource(int64(*s)))
	if *gb {
		genBench(*n, *s, *level)
	} else {
		dumpPng = *d
		if dumpPng > 0 {
			images = loadImages()
		}
		dumpText = *dt
		if dumpText {
			prefix := *dtFile
			successCases, _ = os.Create(prefix + "_success.txt")
			failedCases, _ = os.Create(prefix + "_failed.txt")
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
	for i := 0; i < n; i++ {
		bc := benchCase(level)
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

func benchCase(level int) []int {
	lc := config.Levels[level-1]
	row := lc.Row
	col := lc.Column
	tiles := row * col
	mine := lc.Mine
	res := make([]int, mine)
	mineCandidates := make([]int, tiles)
	for i := 0; i < tiles; i++ {
		mineCandidates[i] = i
	}
	initClick := toIndex(initx, inity, col)
	for i := 0; i < mine; {
		mineTile := rng.Intn(len(mineCandidates))
		/* According to winmine game implementation,
		the board will be re-shuffled if the first click is a mine.
		So we fix the first click and never put a mine on it.
		*/
		if mineCandidates[mineTile] == initClick {
			continue
		}
		res[i] = mineCandidates[mineTile]
		i++
		mineCandidates = remove(mineCandidates, mineTile)
	}
	return res
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

func verboseLog(format string, args ...interface{}) {
	if config.Verbose {
		fmt.Printf(format, args...)
	}
}

const (
	dumpNone = iota
	dumpWorthyFailure
	dumpAll
	dumpVideo
)

func newGstats() map[string]*stats {
	res := make(map[string]*stats)
	m := []string{"first", "random", "corner", "min", "isle"}
	for _, e := range m {
		res[e] = &stats{success: 0, failure: 0}
	}
	return res
}

func runBench(filename string) {
	var total, sure, guess, totalClicks int
	wins := make([]int, len(config.Levels))
	loses := make([]int, len(config.Levels))
	text, err := ioutil.ReadFile(filename)
	check(err)
	gstats := newGstats()
	lines := strings.Split(string(text), "\n")
	t1 := time.Now()
	for i := range lines {
		mines := strings.Split(lines[i], ",")
		//ignore empty lines
		if len(mines) > 1 {
			total++
			board := initBoard(toInt(mines))
			board.text = lines[i]
			verboseLog("%v\n", board)
			if dumpPng == dumpAll {
				board.dump(fmt.Sprintf("%d.png", i))
			}
			player := initPlayer(board, i)
			res := player.play(board)
			if res == tsWin {
				wins[board.level-1]++
				successCases.WriteString(lines[i] + "\n")
			} else {
				if dumpPng >= dumpWorthyFailure && player.worthDump() {
					player.dump(fmt.Sprintf("f%s.png", player.gamename))
				}
				loses[board.level-1]++
				failedCases.WriteString(lines[i] + "\n")
			}
			sure += player.sure
			guess += player.guess
			totalClicks += (player.sure + player.guess)
			for k, v := range player.guessStats {
				gstats[k].success += v.success
				gstats[k].failure += v.failure
			}
		}
	}
	t2 := time.Now()
	spent := t2.Sub(t1).Milliseconds()
	win := sumSlice(wins)
	lose := sumSlice(loses)
	fmt.Printf("win: %.2f (%d, %d, %d), lose: %.2f\n",
		float64(win)/float64(total), wins[0], wins[1], wins[2],
		float64(lose)/float64(total))
	fmt.Printf("sure: %d(%.2f), guess: %d(%.2f), %d clicks in %d ms (%d clicks/s)\n",
		sure,
		float64(sure)/float64(totalClicks),
		guess,
		float64(guess)/float64(totalClicks),
		clickCounter,
		spent,
		int(float64(clickCounter)/float64(spent)*1000))
	for k, v := range gstats {
		fmt.Printf("%s: success = %d\n", k, v.success)
		fmt.Printf("%s: failure = %d\n", k, v.failure)
	}
}

type tileInt int
type board struct {
	tiles  [][]tileInt
	level  int
	row    int
	col    int
	mine   int
	status int
	text   string
}

const (
	initx = 0
	inity = 0
)

func initBoard(mines []int) *board {
	verboseLog("init board with %v %v\n", mines, len(mines))
	level, err := getLevel(len(mines))
	check(err)
	b := &board{level: level, status: 0}
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

func (b *board) setMines(mines []int) {
	b.initTiles()
	for _, mine := range mines {
		x, y := toXY(mine, b.col)
		b.tiles[y][x] = -1
	}
	b.setHints()
}

func (b *board) setHints() {
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

func (b *board) getNeighbors(x, y int) []*tileInt {
	r, c := b.row, b.col
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

func toXY(index, column int) (int, int) {
	return index % column, index / column
}

func toIndex(x, y, column int) int {
	return y*column + x
}

func (b *board) initTiles() {
	c := config.Levels[b.level-1]
	b.row, b.col, b.mine = c.Row, c.Column, c.Mine
	b.tiles = make([][]tileInt, b.row)
	for i := range b.tiles {
		t := make([]tileInt, b.col)
		for j := range t {
			t[j] = 0
		}
		b.tiles[i] = t
	}
}

func (b *board) String() string {
	var buf bytes.Buffer
	for y := range b.tiles {
		fmt.Println(&buf, b.tiles[y])
	}
	return buf.String()
}

func (b *board) dump(outpath string) {
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

func (t *tileInt) isMine() bool {
	return *t == -1
}

func getLevel(n int) (level int, err error) {
	for i := 0; i < len(config.Levels); i++ {
		if n == config.Levels[i].Mine {
			return i + 1, err
		}
	}
	return level, errors.New("Level not found")
}

type tileExt struct {
	value    int
	revealed bool
}

const (
	//tile statuses
	tsUnknown = 10 + iota
	tsBoom    //mine clicked, game over
	tsFlag    //mine marked
	tsWin
	tsLose
)

type player struct {
	tiles      [][]tileExt
	view       map[*intset.IntSet]int
	sure       int
	guess      int
	row        int
	col        int
	mine       int
	gamename   string
	guesser    func() (int, int)
	guessed    string
	guessStats map[string]*stats
}

func initPlayer(b *board, i int) *player {
	verboseLog("init player with level %d\n", b.level)
	p := &player{
		sure:     0,
		guess:    0,
		gamename: fmt.Sprintf("%d-%d", b.level, i),
		guessed:  "none"}
	p.init(b)
	return p
}

func (p *player) init(b *board) {
	p.row, p.col, p.mine = b.row, b.col, b.mine
	p.tiles = make([][]tileExt, p.row)
	for i := range p.tiles {
		t := make([]tileExt, p.col)
		for j := range t {
			t[j].value = tsUnknown
			t[j].revealed = false
		}
		p.tiles[i] = t
	}

	p.guessStats = newGstats()
}

func (p *player) play(b *board) int {
	step := 0
	clickF := func(x, y int) {
		if p.tiles[y][x].value == tsUnknown {
			verboseLog("### step %d ###\n", step)
			if dumpPng == dumpAll {
				p.dump(fmt.Sprintf("f%s-%d.png", p.gamename, step))
			} else if dumpPng == dumpVideo {
				p.dump(fmt.Sprintf("vs-%d.png", pngCounter))
				pngCounter++
			}
			p.click(b, x, y)
			clickCounter++
			step++
		}
	}
	for b.status == 0 {
		if p.guessed != "none" {
			p.guessStats[p.guessed].success++
			p.guessed = "none"
		}
		if p.sure == 0 {
			clickF(initx, inity)
			p.sure++
			continue
		}
		safe := p.findSafe()
		if p.mine == 0 {
			b.status = tsWin
			break
		}
		for _, v := range safe {
			clickF(toXY(v, p.col))
			if p.guessed == "none" {
				p.sure++
			} else {
				p.guess++
			}
		}
		if len(safe) == 0 {
			verboseLog("now we have to guess...\n")
			x, y := p.doGuess()
			verboseLog("guess at (%d, %d)\n", x, y)
			clickF(x, y)
			p.guess++
			continue
		}
	}
	if dumpPng == dumpVideo {
		p.dump(fmt.Sprintf("vs-%d.png", pngCounter))
		pngCounter++
	}
	if b.status == tsWin {
		verboseLog("Win!\n")
		if dumpPng == dumpAll {
			p.dump(fmt.Sprintf("f%s-%d.png", p.gamename, step))
		}
		return tsWin
	}
	p.guessStats[p.guessed].failure++
	verboseLog("Lost!\n")
	return tsLose
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

func (p *player) corners() (int, int) {
	if isUnknown(&p.tiles[0][0]) {
		return 0, 0
	}
	if isUnknown(&p.tiles[p.row-1][p.col-1]) {
		return p.col - 1, p.row - 1
	}
	if isUnknown(&p.tiles[p.row-1][0]) {
		return 0, p.row - 1
	}
	if isUnknown(&p.tiles[0][p.col-1]) {
		return p.col - 1, 0
	}
	return -1, -1
}

func (p *player) doGuess() (int, int) {
	//prefer unknown corners
	x, y := p.corners()
	if x >= 0 {
		p.guessed = "corner"
		return x, y
	}

	p.guessed = config.Guess
	if p.guesser != nil {
		return p.guesser()
	}
	m := map[string]func() (int, int){
		"first":  p.firstGuess,
		"random": p.randomGuess,
		"corner": p.cornerGuess,
		"min":    p.minGuess,
	}
	if g, found := m[config.Guess]; found {
		p.guesser = g
	}
	return p.guesser()
}

func (p *player) minGuess() (int, int) {
	pm := make(map[int]float64)
	for s, v := range p.view {
		if v > 0 {
			sz := s.Len()
			for _, e := range s.Elems() {
				x, y := toXY(e, p.col)
				if !isUnknown(&p.tiles[y][x]) {
					log.Fatal(fmt.Sprintf("unexpected known tile: %d, %d, %v", x, y, p.tiles[y][x]))
				}
				prob := float64(v) / float64(sz)
				if pv, found := pm[e]; found {
					//prefer the more risky data
					if pv < prob {
						pm[e] = prob
					}
				} else {
					//only accept tiles which are not too risky and worthy to guess
					if prob < config.Accept {
						pm[e] = prob
					}
				}
			}
		}
	}

	min := 1.0
	res := -1
	for e, v := range pm {
		if v < min {
			min = v
			res = e
		}
	}
	if res >= 0 {
		verboseLog("Pmin = %f\n", min)
		p.guessed = "min"
		return toXY(res, p.col)
	}

	//no tile with low probility is found
	return p.cornerGuess()
}

func (p *player) firstGuess() (int, int) {
	p.guessed = "first"
	return p.one(isUnknown)
}

func (p *player) cornerGuess() (int, int) {
	leftUpper := func() (int, int) {
		return p.one(isUnknown)
	}
	rightBottom := func() (int, int) {
		for y := p.row - 1; y >= 0; y-- {
			for x := p.col - 1; x >= 0; x-- {
				if isUnknown(&p.tiles[y][x]) {
					return x, y
				}
			}
		}
		return -1, -1
	}
	leftBottom := func() (int, int) {
		for y := p.row - 1; y >= 0; y-- {
			for x := 0; x < p.col; x++ {
				if isUnknown(&p.tiles[y][x]) {
					return x, y
				}
			}
		}
		return -1, -1
	}
	rightUpper := func() (int, int) {
		for y := 0; y < p.row; y++ {
			for x := p.col - 1; x >= 0; x-- {
				if isUnknown(&p.tiles[y][x]) {
					return x, y
				}
			}
		}
		return -1, -1
	}
	methods := []func() (int, int){leftUpper, rightBottom, leftBottom, rightUpper}
	p.guessed = "corner"
	return methods[p.guess%4]()
}

func (p *player) randomGuess() (int, int) {
	p.guessed = "random"
	u := p.collect(isUnknown)
	return toXY(u[rng.Intn(len(u))], p.col)
}

func (p *player) refreshView() error {
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

func (p *player) circle(x, y, v int) (*intset.IntSet, int) {
	var s intset.IntSet
	r, c := p.row, p.col

	var f = func(xt, yt int) {
		vt := p.tiles[yt][xt].value
		if vt == tsUnknown {
			s.Add(toIndex(xt, yt, c))
		} else if vt == tsFlag {
			v--
		}
	}
	if x+1 < c {
		f(x+1, y)
	}
	if x-1 >= 0 {
		f(x-1, y)
	}
	if y+1 < r {
		f(x, y+1)
		if x+1 < c {
			f(x+1, y+1)
		}
		if x-1 >= 0 {
			f(x-1, y+1)
		}
	}
	if y-1 >= 0 {
		f(x, y-1)
		if x+1 < c {
			f(x+1, y-1)
		}
		if x-1 >= 0 {
			f(x-1, y-1)
		}
	}
	return &s, v
}

func isUnknown(t *tileExt) bool {
	return t.value == tsUnknown
}

func isNumber(t *tileExt) bool {
	return t.value >= 0 && t.value <= 8
}

func isFlag(t *tileExt) bool {
	return t.value == tsFlag
}

func (p *player) findSafe() []int {
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
			if v == 0 {
				for _, e := range s.Elems() {
					if !inSlice(e, safe) {
						safe = append(safe, e)
					}
				}
				return safe
			}
			if v == s.Len() {
				for _, e := range s.Elems() {
					x, y := toXY(e, p.col)
					verboseLog("%d %d %d\n", x, y, p.tiles[y][x].value)
					if p.tiles[y][x].value != tsFlag {
						p.tiles[y][x].value = tsFlag
						p.mine--
						if p.mine == 0 {
							return p.collect(isUnknown)
						}
						needRefresh = true
					}
				}
			}
		}
		if strategyEnabled("diff") && !needRefresh {
			verboseLog("searching view directly found no safe tiles, start searching diff\n")
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
		if strategyEnabled("reduce") && !stateChanged {
			verboseLog("searching view diff made no state change, start searching reduce\n")
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

	if p.mine > 0 {
		if strategyEnabled("isle") {
			if p.mine < config.Isle.MaxMine {
				safe = p.findIsle()
			} else {
				verboseLog("remained %d mines, skip isle analysis\n", p.mine)
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
func (p *player) viewKey(s *intset.IntSet) *intset.IntSet {
	for s2 := range p.view {
		if s.String() == s2.String() {
			return s2
		}
	}
	return s
}

type isleContext struct {
	player *player
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
				rc(i+1, j+1)
			}
		}
		return
	}
	rc(0, 0)
}

func (ic *isleContext) solve() ([]int, error) {
	solutions := []*intset.IntSet{}
	solMines := []*intset.IntSet{}
	combinations(len(ic.isle), ic.mine,
		func(s []int) {
			for _, e := range s {
				x, y := toXY(ic.isle[e], ic.player.col)
				ic.player.tiles[y][x].value = tsFlag
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
				var y intset.IntSet
				y.AddAll(ic.mines...)
				solMines = append(solMines, &y)
			}
			for _, e := range s {
				x, y := toXY(ic.isle[e], ic.player.col)
				ic.player.tiles[y][x].value = tsUnknown
			}
		})
	if len(solutions) == 0 {
		return nil, errors.New("found no solution")
	}
	if len(solutions) == 1 {
		ic.player.mine = 0
		for _, e := range ic.mines {
			x, y := toXY(e, ic.player.col)
			ic.player.tiles[y][x].value = tsFlag
		}
		return ic.safe, nil
	}

	bs := map[int]int{}
	for _, t := range ic.isle {
		bs[t] = 0
	}
	for _, s := range solMines {
		for _, e := range s.Elems() {
			bs[e]++
		}
	}
	safest := -1
	vmin := 1000000
	for k, v := range bs {
		if v < vmin {
			vmin = v
			safest = k
		}
	}
	if vmin == 0 {
		return []int{safest}, nil
	}
	prob := float64(vmin) / float64(len(solutions))
	if prob < config.Accept {
		verboseLog("PSmin = %f\n", prob)
		ic.player.guessed = "isle"
		return []int{safest}, errors.New("found multiple solutions, guessed the safest one")
	}
	return nil, errors.New("found multiple solutions")
}

func (p *player) isConsistent() bool {
	numbers := p.collect(isNumber)
	for _, n := range numbers {
		x, y := toXY(n, p.col)
		nf := p.neighbors(x, y, isFlag)
		if p.tiles[y][x].value != nf {
			return false
		}
	}
	return true
}

func (p *player) neighbors(x, y int, filter func(*tileExt) bool) int {
	r, c := p.row, p.col
	n := 0
	var f = func(xt, yt int) {
		if filter(&p.tiles[yt][xt]) {
			n++
		}
	}
	if x+1 < c {
		f(x+1, y)
	}
	if x-1 >= 0 {
		f(x-1, y)
	}
	if y+1 < r {
		f(x, y+1)
		if x+1 < c {
			f(x+1, y+1)
		}
		if x-1 >= 0 {
			f(x-1, y+1)
		}
	}
	if y-1 >= 0 {
		f(x, y-1)
		if x+1 < c {
			f(x+1, y-1)
		}
		if x-1 >= 0 {
			f(x-1, y-1)
		}
	}
	return n
}

func (p *player) findIsle() []int {
	verboseLog("remained mines=%d, locating the isle\n", p.mine)
	empty := []int{}
	x, y := p.one(isUnknown)
	isle := p.isleAt(x, y)
	us := p.collect(isUnknown)
	if len(us) == len(isle) {
		verboseLog("only one isle located: %v.\n", isle)
		if len(isle) > config.Isle.MaxSize {
			verboseLog("isle too large. giving up\n")
			return empty
		}
		verboseLog("Try mines (%d) simulations\n", p.mine)
		ic := &isleContext{player: p, mine: p.mine, isle: isle}
		safe, err := ic.solve()
		if err != nil {
			verboseLog("isle: %v\n", err)
			if safe == nil {
				return empty
			}
		}
		return safe
	}
	return empty
}

func (p *player) isleAt(x, y int) []int {
	visited := make(map[int]bool)
	result := []int{}
	p.isle0(x, y, &result, visited)
	return result
}

func (p *player) isle0(x, y int, result *[]int, visited map[int]bool) {
	if y >= p.row || y < 0 || x < 0 || x >= p.col {
		return
	}
	i := toIndex(x, y, p.col)
	if visited[i] {
		return
	}
	visited[i] = true
	if p.tiles[y][x].value == tsUnknown {
		*result = append(*result, i)
		p.isle0(x-1, y, result, visited)
		p.isle0(x+1, y, result, visited)
		p.isle0(x, y-1, result, visited)
		p.isle0(x, y+1, result, visited)
	}
}

func (p *player) one(filter func(*tileExt) bool) (int, int) {
	for y := range p.tiles {
		for x := range p.tiles[y] {
			if filter(&p.tiles[y][x]) {
				return x, y
			}
		}
	}
	return -1, -1
}

func (p *player) collect(filter func(*tileExt) bool) []int {
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

func (p *player) click(b *board, x, y int) {
	if y >= b.row || y < 0 || x < 0 || x >= b.col {
		return
	}
	if p.tiles[y][x].revealed {
		return
	}
	verboseLog("click at (%d, %d)\n", x, y)
	t := b.tiles[y][x]
	p.tiles[y][x].value = int(t)
	p.tiles[y][x].revealed = true
	if t.isMine() {
		p.tiles[y][x].value = tsBoom
		b.status = tsBoom
		verboseLog("boom at (%d, %d)\n", x, y)
		return
	}

	if t == 0 {
		p.click(b, x-1, y)
		p.click(b, x-1, y-1)
		p.click(b, x+1, y)
		p.click(b, x+1, y+1)
		p.click(b, x, y-1)
		p.click(b, x+1, y-1)
		p.click(b, x, y+1)
		p.click(b, x-1, y+1)
	}
}

func (p *player) worthDump() bool {
	return p.sure >= 10
}

func (p *player) dump(outpath string) {
	file, err := os.Create(outpath)
	check(err)
	defer file.Close()
	width, height := 16*p.col, 16*p.row
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range p.tiles {
		for x, t := range p.tiles[y] {
			imgFile := "unknown.png"
			v := t.value
			if v >= 0 && v <= 8 {
				imgFile = fmt.Sprintf("%d.png", v)
			} else if v == tsBoom {
				imgFile = "bomb_red.png"
			} else if v == tsFlag {
				imgFile = "flag.png"
			}
			render(img, 16*x, 16*y, imgFile)
		}
	}
	png.Encode(file, img)
}

func strategyEnabled(name string) bool {
	for _, s := range config.Strategies {
		if s == name {
			return true
		}
	}
	return false
}
