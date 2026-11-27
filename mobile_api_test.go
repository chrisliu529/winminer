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
	if len(levels) != 2 || levels[0] != (levelOption{1, 8, 8, 10}) || levels[1] != (levelOption{2, 10, 10, 20}) {
		t.Fatalf("unexpected phone levels: %+v", levels)
	}
	for level := 1; level <= 2; level++ {
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
		if mines != phoneLevels[level-1].Mine {
			t.Fatalf("got %d mines", mines)
		}
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
