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
