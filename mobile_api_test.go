package main

import (
	"encoding/json"
	"math/bits"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

func TestMobileDispatchKeepsMineLayoutHidden(t *testing.T) {
	testWebHandler(t)
	server := newMobileServer()

	result, err := dispatchMobileRequest(server, mobileRequest{Action: "newGame", Level: 1})
	if err != nil {
		t.Fatal(err)
	}
	state := result.(map[string]interface{})["state"].(gameSnapshot)
	if len(state.Cells) != 81 {
		t.Fatalf("got %d cells, want 81", len(state.Cells))
	}
	for i, cell := range state.Cells {
		if cell.State != "unknown" || cell.Value != nil {
			t.Fatalf("new game exposes cell %d: %+v", i, cell)
		}
	}

	response, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(response), "minePositions") || strings.Contains(string(response), "seed") {
		t.Fatalf("response contains hidden board data: %s", response)
	}
}

func TestMobileDispatchRejectsInvalidActions(t *testing.T) {
	testWebHandler(t)
	server := newMobileServer()

	if _, err := dispatchMobileRequest(server, mobileRequest{Action: "newGame", Level: 99}); err == nil {
		t.Fatal("expected invalid level to be rejected")
	}
	if _, err := dispatchMobileRequest(server, mobileRequest{Action: "click", GameID: "missing", X: 0, Y: 0}); err == nil {
		t.Fatal("expected missing game to be rejected")
	}
}

func TestPhoneLevelsAndFirstClickSafety(t *testing.T) {
	testWebHandler(t)
	server := newMobileServer()
	result, err := dispatchMobileRequest(server, mobileRequest{Action: "getLevels", Phone: true})
	if err != nil {
		t.Fatal(err)
	}
	levels := result.(map[string]interface{})["levels"].([]levelOption)
	if len(levels) != 3 || levels[0] != (levelOption{1, 8, 8, 10}) || levels[1] != (levelOption{2, 10, 10, 20}) || levels[2] != (levelOption{3, 14, 10, 36}) {
		t.Fatalf("unexpected phone levels: %+v", levels)
	}
	for level := 1; level <= 3; level++ {
		game, err := newRandomPhoneGame(level)
		if err != nil {
			t.Fatal(err)
		}
		var x, y int
		for row := range game.board.tiles {
			for col, tile := range game.board.tiles[row] {
				if tile.isMine() {
					x, y = col, row
					break
				}
			}
		}
		if err := game.click(x, y); err != nil {
			t.Fatal(err)
		}
		if game.board.status == tsBoom || game.board.tiles[y][x].isMine() {
			t.Fatal("first click hit a mine")
		}
		mines := 0
		for _, row := range game.board.tiles {
			for _, tile := range row {
				if tile.isMine() {
					mines++
				}
			}
		}
		if mines != levels[level-1].Mines {
			t.Fatalf("got %d mines", mines)
		}
	}
}

func TestMobileBotStepReturnsClickedCell(t *testing.T) {
	testWebHandler(t)
	server := newMobileServer()
	result, err := dispatchMobileRequest(server, mobileRequest{Action: "newGame", Level: 3, Phone: true})
	if err != nil {
		t.Fatal(err)
	}
	before := result.(map[string]interface{})["state"].(gameSnapshot)
	if before.Rows != 14 || before.Columns != 10 || before.Mines != 36 || before.RemainingMines != 36 {
		t.Fatalf("unexpected advanced game: %+v", before)
	}
	result, err = dispatchMobileRequest(server, mobileRequest{Action: "botStep", GameID: before.ID})
	if err != nil {
		t.Fatal(err)
	}
	response, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		State gameSnapshot `json:"state"`
		Move  struct {
			X *int `json:"x"`
			Y *int `json:"y"`
		} `json:"move"`
	}
	if err := json.Unmarshal(response, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Move.X == nil || decoded.Move.Y == nil {
		t.Fatal("bot response must contain both coordinates")
	}
	x, y := *decoded.Move.X, *decoded.Move.Y
	if x < 0 || x >= before.Columns || y < 0 || y >= before.Rows {
		t.Fatalf("invalid bot coordinates: %d,%d", x, y)
	}
	index := y*before.Columns + x
	if before.Cells[index].State != "unknown" || decoded.State.Cells[index].State != "revealed" {
		t.Fatalf("bot move did not reveal the reported cell: before=%+v after=%+v", before.Cells[index], decoded.State.Cells[index])
	}
}

func TestPhoneHintDoesNotTrustFlagsOrChangeGame(t *testing.T) {
	testWebHandler(t)
	server := newMobileServer()
	board := &board{}
	board.initTilesForConfig(levelConfig{Row: 1, Column: 3, Mine: 1})
	board.tiles[0][2] = -1
	board.setHints()
	bot := newPlayer(1, 3, 1, 1, 0)
	bot.tiles[0][1] = tileExt{value: 1, revealed: true}
	bot.tiles[0][0].value = tsFlag // This flag is incorrect.
	bot.mine--
	game := &webGame{id: "hint-game", board: board, player: bot, phone: true, opened: true}
	server.games[game.id] = game
	before := game.snapshotLocked()
	beforeTiles := append([]tileExt(nil), bot.tiles[0]...)
	for i := 0; i < 2; i++ {
		result, err := dispatchMobileRequest(server, mobileRequest{Action: "hint", GameID: game.id})
		if err != nil {
			t.Fatal(err)
		}
		response := result.(map[string]interface{})
		if response["gameId"] != game.id || response["hint"] != (mobileHint{X: 2, Y: 0, Kind: "guess"}) {
			t.Fatalf("unexpected hint: %+v", response)
		}
		if !reflect.DeepEqual(before, game.snapshotLocked()) ||
			!reflect.DeepEqual(beforeTiles, bot.tiles[0]) || bot.mine != 0 {
			t.Fatal("hint changed the game")
		}
	}
	if err := game.toggleFlag(0, 0); err != nil {
		t.Fatal(err)
	}
	if err := game.click(2, 0); err != nil {
		t.Fatal(err)
	}
	if game.board.status != tsBoom {
		t.Fatal("hint altered the mine layout")
	}
}

func TestPhoneHintClassifiesProofAndIsleGuess(t *testing.T) {
	testWebHandler(t)
	config.Strategies = []string{"diff", "reduce", "isle"}
	config.Isle = isleConf{MaxMine: 9, MaxSize: 19}
	config.Accept = 0.4
	tests := []struct {
		name                                          string
		rows, columns, mineX, mineY, revealX, revealY int
		want                                          mobileHint
	}{
		{"proven safe", 1, 3, 1, 0, 0, 0, mobileHint{X: 2, Y: 0, Kind: "safe"}},
		{"isle guess", 2, 2, 1, 1, 0, 0, mobileHint{Kind: "guess"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			board := &board{}
			board.initTilesForConfig(levelConfig{Row: tt.rows, Column: tt.columns, Mine: 1})
			board.tiles[tt.mineY][tt.mineX] = -1
			board.setHints()
			bot := newPlayer(tt.rows, tt.columns, 1, 1, 0)
			bot.tiles[tt.revealY][tt.revealX] = tileExt{
				value: int(board.tiles[tt.revealY][tt.revealX]), revealed: true,
			}
			game := &webGame{board: board, player: bot, phone: true, opened: true}
			before := game.snapshotLocked()
			hint, err := game.hint()
			if err != nil {
				t.Fatal(err)
			}
			if hint.Kind != tt.want.Kind || (tt.want.Kind == "safe" && hint != tt.want) {
				t.Fatalf("got %+v, want %+v", hint, tt.want)
			}
			if bot.tiles[hint.Y][hint.X].revealed {
				t.Fatalf("hint points to revealed tile: %+v", hint)
			}
			if !reflect.DeepEqual(before, game.snapshotLocked()) {
				t.Fatal("hint changed the game snapshot")
			}
		})
	}
}

func TestPhoneHintFirstClickAndUnavailableTiles(t *testing.T) {
	testWebHandler(t)
	b := &board{}
	b.initTilesForConfig(levelConfig{Row: 1, Column: 2, Mine: 1})
	b.tiles[0][0] = -1
	b.setHints()
	p := newPlayer(1, 2, 1, 1, 0)
	p.random = rand.New(rand.NewSource(1))
	g := &webGame{board: b, player: p, phone: true}
	hint, err := g.hint()
	if err != nil || hint != (mobileHint{X: 0, Y: 0, Kind: "safe"}) {
		t.Fatalf("first-click hint: %+v, %v", hint, err)
	}
	if err := g.click(hint.X, hint.Y); err != nil || g.board.status == tsBoom {
		t.Fatalf("first-click hint was not safe: %v", err)
	}

	g.board.status = 0
	g.opened = true
	p.tiles[0][0] = tileExt{value: 1, revealed: true}
	p.tiles[0][1] = tileExt{value: tsFlag}
	if _, err := g.hint(); err == nil {
		t.Fatal("expected an error when there are no unflagged tiles")
	}
	g.board.status = tsBoom
	if _, err := g.hint(); err == nil {
		t.Fatal("expected an error after the game ends")
	}
}

func TestPhoneSafeHintsMatchSmallBoardsWithIncorrectFlags(t *testing.T) {
	testWebHandler(t)
	config.Strategies = []string{"diff", "reduce", "isle"}
	config.Isle = isleConf{MaxMine: 9, MaxSize: 19}
	config.Accept = 0.4
	safeHints, guesses := 0, 0
	for firstMine := 0; firstMine < 9; firstMine++ {
		for secondMine := firstMine + 1; secondMine < 9; secondMine++ {
			b := &board{}
			b.initTilesForConfig(levelConfig{Row: 3, Column: 3, Mine: 2})
			b.tiles[firstMine/3][firstMine%3] = -1
			b.tiles[secondMine/3][secondMine%3] = -1
			b.setHints()
			for revealed := 1; revealed < 1<<9; revealed++ {
				if bits.OnesCount(uint(revealed)) > 2 ||
					revealed&(1<<firstMine|1<<secondMine) != 0 {
					continue
				}
				for _, flagMine := range []bool{false, true} {
					p := newPlayer(3, 3, 2, 1, 0)
					for i := 0; i < 9; i++ {
						if revealed&(1<<i) != 0 {
							p.tiles[i/3][i%3] = tileExt{value: int(b.tiles[i/3][i%3]), revealed: true}
						}
					}
					for i := 0; i < 9; i++ {
						if revealed&(1<<i) == 0 && b.tiles[i/3][i%3].isMine() == flagMine {
							p.tiles[i/3][i%3].value = tsFlag
							p.mine--
							break
						}
					}
					g := &webGame{board: b, player: p, phone: true, opened: true}
					hint, err := g.hint()
					if err != nil {
						t.Fatalf("mines %d,%d, revealed %09b: %v", firstMine, secondMine, revealed, err)
					}
					if !isUnknown(&p.tiles[hint.Y][hint.X]) {
						t.Fatalf("hint selected flagged or revealed tile: %+v", hint)
					}
					switch hint.Kind {
					case "safe":
						safeHints++
						if b.tiles[hint.Y][hint.X].isMine() {
							t.Fatalf("unsafe hint %+v: mines %d,%d, revealed %09b, flagMine %t", hint, firstMine, secondMine, revealed, flagMine)
						}
					case "guess":
						guesses++
					default:
						t.Fatalf("invalid hint kind: %+v", hint)
					}
				}
			}
		}
	}
	if safeHints == 0 || guesses == 0 {
		t.Fatalf("expected both hint kinds, got %d safe and %d guesses", safeHints, guesses)
	}
}

func TestPhoneSnapshotShowsMinesOnlyAfterGameEnds(t *testing.T) {
	testWebHandler(t)
	game, err := newRandomPhoneGame(1)
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []int{0, tsBoom, tsWin} {
		game.board.status = status
		count := 0
		for _, cell := range game.snapshot().Cells {
			if cell.State == "mine" {
				count++
			}
		}
		want := 10
		if status == 0 {
			want = 0
		}
		if count != want {
			t.Fatalf("status %d: got %d visible mines, want %d", status, count, want)
		}
	}
}

func TestPhoneFloodDoesNotRevealFlags(t *testing.T) {
	testWebHandler(t)
	game, err := newRandomPhoneGame(1)
	if err != nil {
		t.Fatal(err)
	}
	game.board.initTilesForConfig(phoneLevels[0])
	game.board.tiles[7][7] = -1
	game.board.setHints()
	game.opened = true
	if err := game.toggleFlag(1, 0); err != nil {
		t.Fatal(err)
	}
	if err := game.click(0, 0); err != nil {
		t.Fatal(err)
	}
	if game.player.tiles[0][1].value != tsFlag || game.player.tiles[0][1].revealed {
		t.Fatal("flood reveal removed a flag")
	}
}
