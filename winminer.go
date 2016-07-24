package main

import (
	"flag"
	"fmt"
	"math/rand"
)

type LevelConfig struct {
	row    int
	column int
	bomb   int
}

var (
	levelConfigs = []LevelConfig{
		{9, 9, 10},
	}
)

func main() {
	gb := flag.Bool("gb", false, "generate benchmark cases or not")
	flag.Parse()

	if *gb {
		n := flag.Int("n", 1, "number of benchmark cases")
		s := flag.Int("s", 0, "random seed for generating benchmark cases")
		level := flag.Int("lv", 1, "game level (1-3)")
		genBench(*n, *s, *level)
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
	bomb := lc.bomb
	bombCandidates := make([]int, tiles)
	for i := 0; i < tiles; i++ {
		bombCandidates[i] = i
	}
	for i := 0; i < bomb; i++ {
		bombTile := rng.Intn(len(bombCandidates))
		if i == 0 {
			fmt.Printf("%d", bombTile)
		} else {
			fmt.Printf(",%d", bombTile)
		}
		remove(bombCandidates, bombTile)
	}
	fmt.Print("\n")
}

func remove(slice []int, i int) []int {
	copy(slice[i:], slice[i+1:])
	return slice[:len(slice)-1]
}
