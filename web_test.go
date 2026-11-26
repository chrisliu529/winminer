package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testWebHandler(t *testing.T) http.Handler {
	t.Helper()
	oldConfig := config
	config = tomlConfig{
		Levels:     []levelConfig{{Row: 9, Column: 9, Mine: 10}},
		Strategies: []string{"diff"},
		Guess:      "first",
		Accept:     0.3,
		Isle:       isleConf{MaxMine: 5, MaxSize: 10},
	}
	t.Cleanup(func() {
		config = oldConfig
	})
	handler, err := newWebHandler()
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func jsonRequest(method, target, body string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	return request
}

func TestWebGameOnlyReturnsObservedTiles(t *testing.T) {
	handler := testWebHandler(t)
	request := jsonRequest(http.MethodPost, "/api/games", `{"level":1}`)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create game returned %d: %s", response.Code, response.Body.String())
	}

	var result struct {
		State gameSnapshot `json:"state"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.State.Cells) != 81 {
		t.Fatalf("got %d cells, want 81", len(result.State.Cells))
	}
	for i, cell := range result.State.Cells {
		if cell.State != "unknown" || cell.Value != nil {
			t.Fatalf("new game exposes cell %d: %+v", i, cell)
		}
	}
	if strings.Contains(response.Body.String(), "board") || strings.Contains(response.Body.String(), "minePositions") {
		t.Fatalf("response contains hidden board fields: %s", response.Body.String())
	}
}

func TestWebFirstClickCanHitMine(t *testing.T) {
	testWebHandler(t)
	var game *webGame
	for seed := int64(0); seed < 1000; seed++ {
		candidate, err := newWebGame("test", 1, seed)
		if err != nil {
			t.Fatal(err)
		}
		if candidate.board.tiles[0][0].isMine() {
			game = candidate
			break
		}
	}
	if game == nil {
		t.Fatal("did not find a board with a mine at the first tile")
	}

	if err := game.click(0, 0); err != nil {
		t.Fatal(err)
	}
	if got := game.snapshotLocked().Status; got != "lost" {
		t.Fatalf("first click status is %q, want lost", got)
	}
	if got := game.snapshotLocked().Cells[0].State; got != "exploded" {
		t.Fatalf("first clicked tile state is %q, want exploded", got)
	}
}

func TestWebLevelsAndSeedValidation(t *testing.T) {
	handler := testWebHandler(t)
	levels := httptest.NewRecorder()
	handler.ServeHTTP(levels, httptest.NewRequest(http.MethodGet, "/api/levels", nil))
	if levels.Code != http.StatusOK || !strings.Contains(levels.Body.String(), `"level":1`) {
		t.Fatalf("unexpected levels response: %d %s", levels.Code, levels.Body.String())
	}

	create := jsonRequest(http.MethodPost, "/api/games", `{"level":1,"seed":7}`)
	createResponse := httptest.NewRecorder()
	handler.ServeHTTP(createResponse, create)
	if createResponse.Code != http.StatusBadRequest {
		t.Fatalf("client-selected seed returned %d, want %d", createResponse.Code, http.StatusBadRequest)
	}
}

func TestWebBotStepAndInvalidCoordinates(t *testing.T) {
	handler := testWebHandler(t)
	create := jsonRequest(http.MethodPost, "/api/games", `{"level":1}`)
	createResponse := httptest.NewRecorder()
	handler.ServeHTTP(createResponse, create)

	var created struct {
		State gameSnapshot `json:"state"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	step := jsonRequest(http.MethodPost, "/api/games/"+created.State.ID+"/bot/step", "{}")
	stepResponse := httptest.NewRecorder()
	handler.ServeHTTP(stepResponse, step)
	if stepResponse.Code != http.StatusOK {
		t.Fatalf("bot step returned %d: %s", stepResponse.Code, stepResponse.Body.String())
	}

	click := jsonRequest(http.MethodPost, "/api/games/"+created.State.ID+"/click", `{"x":99,"y":0}`)
	clickResponse := httptest.NewRecorder()
	handler.ServeHTTP(clickResponse, click)
	if clickResponse.Code != http.StatusBadRequest {
		t.Fatalf("out-of-range click returned %d, want %d", clickResponse.Code, http.StatusBadRequest)
	}

	remove := httptest.NewRequest(http.MethodDelete, "/api/games/"+created.State.ID, nil)
	removeResponse := httptest.NewRecorder()
	handler.ServeHTTP(removeResponse, remove)
	if removeResponse.Code != http.StatusNoContent {
		t.Fatalf("delete game returned %d, want %d", removeResponse.Code, http.StatusNoContent)
	}
}

func TestFindSafeReturnsInvalidObservationError(t *testing.T) {
	testWebHandler(t)
	bot := newPlayer(3, 3, 1, 1, 0)
	bot.tiles[1][1] = tileExt{value: 1, revealed: true}
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			if x != 1 || y != 1 {
				bot.tiles[y][x].value = tsFlag
			}
		}
	}
	if _, err := bot.findSafe(); err == nil {
		t.Fatal("expected inconsistent observation to return an error")
	}
}

func TestWebServesFrontend(t *testing.T) {
	handler := testWebHandler(t)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("frontend returned %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "Winminer") {
		t.Fatal("frontend response does not contain the page title")
	}
}
