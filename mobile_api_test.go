package main

import (
	"encoding/json"
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
