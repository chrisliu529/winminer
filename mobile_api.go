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
	case "click", "toggleFlag", "botStep":
		game, err := findMobileGame(server, request.GameID)
		if err != nil {
			return nil, err
		}
		game.mu.Lock()
		defer game.mu.Unlock()
		switch request.Action {
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
