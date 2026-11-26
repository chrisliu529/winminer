const boardElement = document.querySelector("#board");
const statusElement = document.querySelector("#status");
const messageElement = document.querySelector("#message");
const levelElement = document.querySelector("#level");
const newGameButton = document.querySelector("#new-game");
const botStepButton = document.querySelector("#bot-step");
const autoButton = document.querySelector("#auto");

let game = null;
let busy = false;
let autoplay = false;
let autoplayToken = 0;

async function request(path, options = {}) {
  const response = await fetch(path, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...options.headers,
    },
  });
  if (response.status === 204) {
    return null;
  }
  const body = await response.json();
  if (!response.ok) {
    throw new Error(body.error || `Request failed (${response.status})`);
  }
  return body;
}

function showError(error) {
  messageElement.textContent = error.message;
}

function render() {
  if (!game) {
    return;
  }

  boardElement.classList.toggle("is-busy", busy);
  boardElement.setAttribute("aria-busy", String(busy));
  const columns = String(game.columns);
  if (
    boardElement.childElementCount !== game.cells.length ||
    boardElement.dataset.columns !== columns
  ) {
    const cells = document.createDocumentFragment();
    for (let index = 0; index < game.cells.length; index++) {
      const x = index % game.columns;
      const y = Math.floor(index / game.columns);
      const button = document.createElement("button");
      button.type = "button";
      button.addEventListener("click", () => performAction("click", x, y));
      button.addEventListener("contextmenu", (event) => {
        event.preventDefault();
        performAction("flag", x, y);
      });
      cells.append(button);
    }
    boardElement.replaceChildren(cells);
    boardElement.dataset.columns = columns;
  }

  boardElement.style.gridTemplateColumns = `repeat(${game.columns}, 32px)`;
  for (let index = 0; index < game.cells.length; index++) {
    const cell = game.cells[index];
    const x = index % game.columns;
    const y = Math.floor(index / game.columns);
    const button = boardElement.children[index];
    button.className = `cell ${cell.state}`;
    button.setAttribute("aria-label", cellLabel(cell, x, y));
    button.disabled = game.status !== "playing" || cell.state === "revealed" || cell.state === "exploded";

    if (cell.state === "revealed") {
      button.textContent = cell.value === 0 ? "" : String(cell.value);
      if (cell.value > 0) {
        button.classList.add(`number-${cell.value}`);
      }
    } else if (cell.state === "flag") {
      button.textContent = "⚑";
    } else if (cell.state === "exploded") {
      button.textContent = "✹";
    } else {
      button.textContent = "";
    }
  }

  const stateText = game.status === "playing"
    ? `Playing · ${game.remainingMines} mines remaining`
    : game.status === "won" ? "You won!" : "Game over";
  statusElement.textContent = `${stateText} · ${game.mines} total mines`;
  botStepButton.disabled = busy || game.status !== "playing";
  newGameButton.disabled = busy;
  autoButton.disabled = game.status !== "playing" && !autoplay;
  autoButton.textContent = autoplay ? "Pause" : "Auto play";
}

function cellLabel(cell, x, y) {
  if (cell.state === "revealed") {
    return cell.value === 0 ? `Empty at ${x + 1}, ${y + 1}` : `${cell.value} at ${x + 1}, ${y + 1}`;
  }
  return `${cell.state} at ${x + 1}, ${y + 1}`;
}

async function performAction(action, x, y) {
  if (!game || busy || game.status !== "playing") {
    return;
  }
  busy = true;
  messageElement.textContent = "";
  render();
  try {
    const result = await request(`/api/games/${game.id}/${action}`, {
      method: "POST",
      body: JSON.stringify({ x, y }),
    });
    game = result.state;
  } catch (error) {
    showError(error);
  } finally {
    busy = false;
    render();
  }
}

async function botStep() {
  if (!game || busy || game.status !== "playing") {
    return;
  }
  busy = true;
  messageElement.textContent = "";
  render();
  try {
    const result = await request(`/api/games/${game.id}/bot/step`, {
      method: "POST",
      body: "{}",
    });
    game = result.state;
  } catch (error) {
    showError(error);
    autoplay = false;
  } finally {
    busy = false;
    render();
  }
}

async function autoTick(token) {
  if (!autoplay || token !== autoplayToken) {
    return;
  }
  await botStep();
  if (token !== autoplayToken) {
    return;
  }
  if (autoplay && game?.status === "playing") {
    window.setTimeout(() => autoTick(token), 180);
  } else {
    autoplay = false;
    render();
  }
}

async function newGame() {
  if (busy) {
    return;
  }
  autoplay = false;
  autoplayToken++;
  busy = true;
  messageElement.textContent = "";
  newGameButton.disabled = true;
  const previousGameID = game?.id;
  try {
    const result = await request("/api/games", {
      method: "POST",
      body: JSON.stringify({ level: Number(levelElement.value) }),
    });
    game = result.state;
    if (previousGameID) {
      await request(`/api/games/${previousGameID}`, { method: "DELETE" });
    }
  } catch (error) {
    showError(error);
  } finally {
    busy = false;
    render();
  }
}

newGameButton.addEventListener("click", newGame);
botStepButton.addEventListener("click", botStep);
autoButton.addEventListener("click", () => {
  autoplay = !autoplay;
  autoplayToken++;
  render();
  if (autoplay) {
    autoTick(autoplayToken);
  }
});

async function initialize() {
  try {
    const result = await request("/api/levels");
    for (const level of result.levels) {
      const option = document.createElement("option");
      option.value = String(level.level);
      option.textContent = `Level ${level.level} · ${level.rows}×${level.columns} · ${level.mines} mines`;
      levelElement.append(option);
    }
    if (result.levels.length === 0) {
      throw new Error("No levels are configured");
    }
    await newGame();
  } catch (error) {
    showError(error);
    statusElement.textContent = "Unable to start a game";
  }
}

initialize();
