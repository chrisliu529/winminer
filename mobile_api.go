package main

import (
	"errors"
	"fmt"
)

type mobileRequest struct {
	Action string `json:"action"`
	Level  int    `json:"level,omitempty"`
	GameID string `json:"gameId,omitempty"`
	X      int    `json:"x,omitempty"`
	Y      int    `json:"y,omitempty"`
}

func dispatchMobileRequest(server *webServer, request mobileRequest) (interface{}, error) {
	switch request.Action {
	case "getLevels":
		levels := make([]levelOption, 0, len(config.Levels))
		for i, level := range config.Levels {
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
		game, err := newRandomWebGame(level)
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
			err = game.botStep()
		}
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"state": game.snapshotLocked()}, nil
	default:
		return nil, fmt.Errorf("unknown action: %s", request.Action)
	}
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
