package main

import (
	"errors"
	"fmt"
	mathrand "math/rand"
)

type mobileRequest struct {
	Action string `json:"action"`
	Level  int    `json:"level,omitempty"`
	GameID string `json:"gameId,omitempty"`
	X      int    `json:"x,omitempty"`
	Y      int    `json:"y,omitempty"`
	Phone  bool   `json:"phone,omitempty"`
}

type mobileHint struct {
	X    int    `json:"x"`
	Y    int    `json:"y"`
	Kind string `json:"kind"`
}

var phoneLevels = []levelConfig{
	{Row: 8, Column: 8, Mine: 10},
	{Row: 10, Column: 10, Mine: 20},
	{Row: 14, Column: 10, Mine: 36},
}

func dispatchMobileRequest(server *webServer, request mobileRequest) (interface{}, error) {
	switch request.Action {
	case "getLevels":
		configuredLevels := config.Levels
		if request.Phone {
			configuredLevels = phoneLevels
		}
		levels := make([]levelOption, 0, len(configuredLevels))
		for i, level := range configuredLevels {
			levels = append(levels, levelOption{
				Level:   i + 1,
				Rows:    level.Row,
				Columns: level.Column,
				Mines:   level.Mine,
			})
		}
		return map[string]interface{}{"levels": levels}, nil
	case "newGame":
		level := request.Level
		if level == 0 {
			level = 1
		}
		var game *webGame
		var err error
		if request.Phone {
			game, err = newRandomPhoneGame(level)
		} else {
			game, err = newRandomWebGame(level)
		}
		if err != nil {
			return nil, err
		}
		server.mu.Lock()
		server.games[game.id] = game
		server.mu.Unlock()
		return map[string]interface{}{"state": game.snapshot()}, nil
	case "deleteGame":
		game, err := findMobileGame(server, request.GameID)
		if err != nil {
			return nil, err
		}
		game.mu.Lock()
		server.mu.Lock()
		if server.games[request.GameID] == game {
			delete(server.games, request.GameID)
		}
		server.mu.Unlock()
		game.mu.Unlock()
		return map[string]interface{}{"ok": true}, nil
	case "click", "toggleFlag", "botStep", "hint":
		game, err := findMobileGame(server, request.GameID)
		if err != nil {
			return nil, err
		}
		game.mu.Lock()
		defer game.mu.Unlock()
		switch request.Action {
		case "hint":
			if !game.phone {
				return nil, errors.New("hints are only available for phone games")
			}
			hint, err := game.hint()
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{"gameId": game.id, "hint": hint}, nil
		case "click":
			err = game.click(request.X, request.Y)
		case "toggleFlag":
			err = game.toggleFlag(request.X, request.Y)
		case "botStep":
			var move coordinateRequest
			move, err = game.botStep()
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{"state": game.snapshotLocked(), "move": move}, nil
		}
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"state": game.snapshotLocked()}, nil
	default:
		return nil, fmt.Errorf("unknown action: %s", request.Action)
	}
}

func (g *webGame) hint() (mobileHint, error) {
	if g.board.status != 0 {
		return mobileHint{}, errors.New("game is already finished")
	}

	// Player flags are hypotheses, not evidence that a tile contains a mine.
	bot := newPlayer(g.player.row, g.player.col, g.board.mine, g.board.level, 0)
	bot.random = mathrand.New(mathrand.NewSource(0))
	for y, row := range g.player.tiles {
		for x, tile := range row {
			if tile.revealed {
				bot.tiles[y][x] = tile
			}
		}
	}
	available := func(x, y int) bool {
		return x >= 0 && x < bot.col && y >= 0 && y < bot.row &&
			isUnknown(&g.player.tiles[y][x]) && isUnknown(&bot.tiles[y][x])
	}
	choose := func(kind string) (mobileHint, bool) {
		corners := [][2]int{{0, 0}, {bot.col - 1, bot.row - 1}, {0, bot.row - 1}, {bot.col - 1, 0}}
		for _, corner := range corners {
			if available(corner[0], corner[1]) {
				return mobileHint{X: corner[0], Y: corner[1], Kind: kind}, true
			}
		}
		for y := 0; y < bot.row; y++ {
			for x := 0; x < bot.col; x++ {
				if available(x, y) {
					return mobileHint{X: x, Y: y, Kind: kind}, true
				}
			}
		}
		return mobileHint{}, false
	}

	first, ok := choose("guess")
	if !ok {
		return mobileHint{}, errors.New("no available tile for a hint")
	}
	if !g.opened {
		first.Kind = "safe"
		return first, nil
	}
	safe, err := bot.findSafe()
	if err != nil {
		return mobileHint{}, err
	}
	for _, index := range safe {
		x, y := toXY(index, bot.col)
		if available(x, y) {
			kind := "safe"
			if bot.guessed == "isle" {
				kind = "guess"
			}
			return mobileHint{X: x, Y: y, Kind: kind}, nil
		}
	}
	if len(safe) == 0 {
		x, y := bot.doGuess()
		if available(x, y) {
			return mobileHint{X: x, Y: y, Kind: "guess"}, nil
		}
	}
	if hint, ok := choose("guess"); ok {
		return hint, nil
	}
	return mobileHint{}, errors.New("no available tile for a hint")
}

func newRandomPhoneGame(level int) (*webGame, error) {
	if level < 1 || level > len(phoneLevels) {
		return nil, errors.New("invalid level")
	}
	id, seed, err := randomGameIdentity()
	if err != nil {
		return nil, err
	}
	random := mathrand.New(mathrand.NewSource(seed))
	settings := phoneLevels[level-1]
	b := &board{level: level}
	b.initTilesForConfig(settings)
	for _, index := range random.Perm(settings.Row * settings.Column)[:settings.Mine] {
		x, y := toXY(index, settings.Column)
		b.tiles[y][x] = -1
	}
	b.setHints()
	p := newPlayer(b.row, b.col, b.mine, level, 0)
	p.random = random
	return &webGame{id: id, board: b, player: p, phone: true}, nil
}

func findMobileGame(server *webServer, id string) (*webGame, error) {
	if id == "" {
		return nil, errors.New("game id is required")
	}
	server.mu.RLock()
	game := server.games[id]
	server.mu.RUnlock()
	if game == nil {
		return nil, errors.New("game not found")
	}
	return game, nil
}

func newMobileServer() *webServer {
	return &webServer{games: make(map[string]*webGame)}
}
