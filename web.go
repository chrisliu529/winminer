package main

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	mathrand "math/rand"
	"mime"
	"net/http"
	"strings"
	"sync"
	"time"
)

//go:embed web/*
var webAssets embed.FS

type webCell struct {
	State string `json:"state"`
	Value *int   `json:"value,omitempty"`
}

type gameSnapshot struct {
	ID             string    `json:"id"`
	Rows           int       `json:"rows"`
	Columns        int       `json:"columns"`
	Mines          int       `json:"mines"`
	RemainingMines int       `json:"remainingMines"`
	Status         string    `json:"status"`
	Cells          []webCell `json:"cells"`
}

type levelOption struct {
	Level   int `json:"level"`
	Rows    int `json:"rows"`
	Columns int `json:"columns"`
	Mines   int `json:"mines"`
}

type webGame struct {
	mu     sync.Mutex
	id     string
	board  *board
	player *player
	phone  bool
	opened bool
}

type webServer struct {
	mu    sync.RWMutex
	games map[string]*webGame
}

type createGameRequest struct {
	Level int `json:"level"`
}

type coordinateRequest struct {
	X int `json:"x"`
	Y int `json:"y"`
}

func serveWeb(addr string) error {
	handler, err := newWebHandler()
	if err != nil {
		return err
	}
	log.Printf("web game available at http://%s", addr)
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return server.ListenAndServe()
}

func newWebHandler() (http.Handler, error) {
	staticFiles, err := fs.Sub(webAssets, "web")
	if err != nil {
		return nil, err
	}

	api := &webServer{games: make(map[string]*webGame)}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/levels", api.handleLevels)
	mux.HandleFunc("/api/games", api.handleCreateGame)
	mux.HandleFunc("/api/games/", api.handleGame)
	mux.Handle("/", http.FileServer(http.FS(staticFiles)))
	return mux, nil
}

func (s *webServer) handleLevels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeWebError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	levels := make([]levelOption, 0, len(config.Levels))
	for i, level := range config.Levels {
		levels = append(levels, levelOption{
			Level:   i + 1,
			Rows:    level.Row,
			Columns: level.Column,
			Mines:   level.Mine,
		})
	}
	writeJSON(w, http.StatusOK, map[string][]levelOption{"levels": levels})
}

func (s *webServer) handleCreateGame(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeWebError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}

	var request createGameRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeWebError(w, http.StatusBadRequest, err)
		return
	}
	if request.Level == 0 {
		request.Level = 1
	}
	if request.Level < 1 || request.Level > len(config.Levels) {
		writeWebError(w, http.StatusBadRequest, errors.New("invalid level"))
		return
	}

	game, err := newRandomWebGame(request.Level)
	if err != nil {
		writeWebError(w, http.StatusInternalServerError, err)
		return
	}
	s.mu.Lock()
	s.games[game.id] = game
	s.mu.Unlock()
	writeJSON(w, http.StatusCreated, map[string]gameSnapshot{"state": game.snapshot()})
}

func (s *webServer) handleGame(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 || parts[0] != "api" || parts[1] != "games" {
		http.NotFound(w, r)
		return
	}
	s.mu.RLock()
	game := s.games[parts[2]]
	s.mu.RUnlock()
	if game == nil {
		writeWebError(w, http.StatusNotFound, errors.New("game not found"))
		return
	}

	if len(parts) == 3 && r.Method == http.MethodGet {
		game.mu.Lock()
		snapshot := game.snapshotLocked()
		game.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]gameSnapshot{"state": snapshot})
		return
	}
	if len(parts) == 3 && r.Method == http.MethodDelete {
		game.mu.Lock()
		s.mu.Lock()
		if s.games[parts[2]] == game {
			delete(s.games, parts[2])
		}
		s.mu.Unlock()
		game.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeWebError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}

	game.mu.Lock()
	defer game.mu.Unlock()
	if len(parts) == 5 && parts[3] == "bot" && parts[4] == "step" {
		if _, err := game.botStep(); err != nil {
			writeWebError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]gameSnapshot{"state": game.snapshotLocked()})
		return
	}
	if len(parts) != 4 {
		http.NotFound(w, r)
		return
	}
	switch parts[3] {
	case "click":
		var request coordinateRequest
		if err := decodeJSON(w, r, &request); err != nil {
			writeWebError(w, http.StatusBadRequest, err)
			return
		}
		if err := game.click(request.X, request.Y); err != nil {
			writeWebError(w, http.StatusBadRequest, err)
			return
		}
	case "flag":
		var request coordinateRequest
		if err := decodeJSON(w, r, &request); err != nil {
			writeWebError(w, http.StatusBadRequest, err)
			return
		}
		if err := game.toggleFlag(request.X, request.Y); err != nil {
			writeWebError(w, http.StatusBadRequest, err)
			return
		}
	default:
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]gameSnapshot{"state": game.snapshotLocked()})
}

func newRandomWebGame(level int) (*webGame, error) {
	if level < 1 || level > len(config.Levels) {
		return nil, errors.New("invalid level")
	}
	id, seed, err := randomGameIdentity()
	if err != nil {
		return nil, err
	}
	return newWebGame(id, level, seed)
}

func randomGameIdentity() (string, int64, error) {
	var idBytes [16]byte
	if _, err := rand.Read(idBytes[:]); err != nil {
		return "", 0, err
	}
	var seedBytes [8]byte
	if _, err := rand.Read(seedBytes[:]); err != nil {
		return "", 0, err
	}
	var seed int64
	for i, b := range seedBytes {
		seed |= int64(b) << (8 * i)
	}
	return hex.EncodeToString(idBytes[:]), seed, nil
}

func newWebGame(id string, level int, seed int64) (*webGame, error) {
	gameRNG := mathrand.New(mathrand.NewSource(seed))
	levelSettings := config.Levels[level-1]
	mines := gameRNG.Perm(levelSettings.Row * levelSettings.Column)[:levelSettings.Mine]
	gameBoard := initBoardForLevel(level, mines)
	bot := newPlayer(gameBoard.row, gameBoard.col, gameBoard.mine, level, 0)
	bot.random = gameRNG

	return &webGame{id: id, board: gameBoard, player: bot}, nil
}

func (g *webGame) click(x, y int) error {
	if g.board.status != 0 {
		return errors.New("game is already finished")
	}
	if y < 0 || y >= g.board.row || x < 0 || x >= g.board.col {
		return errors.New("coordinates out of range")
	}
	if g.player.tiles[y][x].revealed {
		return errors.New("tile is already revealed")
	}
	if g.player.tiles[y][x].value == tsFlag {
		return errors.New("remove the flag before clicking this tile")
	}
	if g.phone && !g.opened {
		g.ensureFirstClickSafe(x, y)
		g.opened = true
	}
	revealed, err := g.reveal(x, y)
	if err != nil {
		return err
	}
	g.player.observe(revealed)
	return nil
}

func (g *webGame) ensureFirstClickSafe(x, y int) {
	if !g.board.tiles[y][x].isMine() {
		return
	}
	b := g.board
	mines := make([]int, 0, b.mine)
	available := make([]int, 0, b.row*b.col-b.mine)
	for row := range b.tiles {
		for col, tile := range b.tiles[row] {
			index := row*b.col + col
			if tile.isMine() && (col != x || row != y) {
				mines = append(mines, index)
			} else if !tile.isMine() {
				available = append(available, index)
			}
		}
	}
	mines = append(mines, available[g.player.random.Intn(len(available))])
	b.initTilesForConfig(levelConfig{Row: b.row, Column: b.col, Mine: b.mine})
	for _, index := range mines {
		mx, my := toXY(index, b.col)
		b.tiles[my][mx] = -1
	}
	b.setHints()
}

func (g *webGame) toggleFlag(x, y int) error {
	if g.board.status != 0 {
		return errors.New("game is already finished")
	}
	if y < 0 || y >= g.board.row || x < 0 || x >= g.board.col {
		return errors.New("coordinates out of range")
	}
	tile := &g.player.tiles[y][x]
	if tile.revealed {
		return errors.New("cannot flag a revealed tile")
	}
	if tile.value == tsFlag {
		tile.value = tsUnknown
		g.player.mine++
		return nil
	}
	if g.player.mine <= 0 {
		return errors.New("no unmarked mines remain")
	}
	tile.value = tsFlag
	g.player.mine--
	return nil
}

func (g *webGame) botStep() (coordinateRequest, error) {
	if g.board.status != 0 {
		return coordinateRequest{}, errors.New("game is already finished")
	}
	x, y, ok, err := g.player.nextAction()
	if err != nil {
		return coordinateRequest{}, err
	}
	if !ok {
		return coordinateRequest{}, errors.New("bot has no available move")
	}
	if g.player.tiles[y][x].value == tsFlag {
		return coordinateRequest{}, errors.New("bot selected a flagged tile")
	}
	if g.phone && !g.opened {
		g.ensureFirstClickSafe(x, y)
		g.opened = true
	}
	revealed, err := g.reveal(x, y)
	if err != nil {
		return coordinateRequest{}, err
	}
	g.player.observe(revealed)
	return coordinateRequest{X: x, Y: y}, nil
}

func (g *webGame) reveal(x, y int) ([]tileReveal, error) {
	if g.phone {
		return g.board.revealWithFlags(x, y, g.player.tiles)
	}
	return g.board.reveal(x, y)
}

func (g *webGame) snapshot() gameSnapshot {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.snapshotLocked()
}

func (g *webGame) snapshotLocked() gameSnapshot {
	cells := make([]webCell, 0, g.board.row*g.board.col)
	for y := range g.player.tiles {
		for x := range g.player.tiles[y] {
			tile := g.player.tiles[y][x]
			cell := webCell{State: "unknown"}
			switch {
			case tile.value == tsBoom:
				cell.State = "exploded"
			case g.phone && g.board.status != 0 && g.board.tiles[y][x].isMine():
				cell.State = "mine"
			case tile.revealed:
				value := tile.value
				cell.State = "revealed"
				cell.Value = &value
			case tile.value == tsFlag:
				cell.State = "flag"
			}
			cells = append(cells, cell)
		}
	}
	return gameSnapshot{
		ID:             g.id,
		Rows:           g.board.row,
		Columns:        g.board.col,
		Mines:          g.board.mine,
		RemainingMines: g.player.mine,
		Status:         gameStatus(g.board.status),
		Cells:          cells,
	}
}

func gameStatus(status int) string {
	switch status {
	case tsBoom:
		return "lost"
	case tsWin:
		return "won"
	default:
		return "playing"
	}
}

func (p *player) observe(revealed []tileReveal) {
	for _, tile := range revealed {
		cell := &p.tiles[tile.y][tile.x]
		if cell.value == tsFlag {
			p.mine++
		}
		cell.revealed = true
		cell.value = tile.value
	}
}

func (p *player) nextAction() (int, int, bool, error) {
	safe, err := p.findSafe()
	if err != nil {
		return 0, 0, false, err
	}
	if len(safe) > 0 {
		x, y := toXY(safe[0], p.col)
		return x, y, true, nil
	}
	unknown := p.collect(isUnknown)
	if len(unknown) == 0 {
		return 0, 0, false, nil
	}
	x, y := p.doGuess()
	if x < 0 || y < 0 {
		return 0, 0, false, nil
	}
	return x, y, true, nil
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dest interface{}) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return errors.New("Content-Type must be application/json")
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return err
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("request must contain one JSON value")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write JSON response: %v", err)
	}
}

func writeWebError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
