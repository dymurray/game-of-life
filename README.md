# Conway's Game of Life (Go)

A minimal implementation of Conway's Game of Life written in Go, with both a terminal mode and an interactive web UI.

## What is Conway's Game of Life?

Conway's Game of Life is a zero-player cellular automaton devised by mathematician John Horton Conway in 1970. The "game" consists of a two-dimensional orthogonal grid of square cells, each of which is in one of two possible states: alive or dead. Every cell interacts with its eight neighbours (the Moore neighbourhood) and the following four rules are applied simultaneously to every cell in each generation:

- **Underpopulation**: Any live cell with fewer than two live neighbours dies.
- **Survival**: Any live cell with two or three live neighbours survives to the next generation.
- **Overpopulation**: Any live cell with more than three live neighbours dies.
- **Reproduction**: Any dead cell with exactly three live neighbours becomes a live cell.

The game evolves from an initial configuration according to these rules; common emergent patterns include still lifes (stable shapes), oscillators (e.g. the blinker), and spaceships (e.g. the glider).

## How this program works

- Written in pure Go using only the standard library.
- `life.go` holds the board logic, `main.go` the terminal mode, and `server.go` + `web/` the web UI.
- The board is represented as a `grid` (`[][]bool`) with a default size of **60×30**, randomly seeded so that approximately 25 % of cells start alive.
- **Toroidal topology**: neighbour counting (`countNeighbors`) wraps around the edges using modulo arithmetic, so patterns that leave one side of the grid reappear on the opposite side.
- `step` computes the next generation into a fresh grid by applying Conway's four rules.
- `render` clears the terminal with the ANSI escape sequence `\033[H\033[2J` and redraws the grid in place, producing an animation running at roughly 10 frames per second.
- The program loops forever until the user presses **Ctrl+C**.

## Usage

### Prerequisites

- Go 1.21 or later.

### Run (terminal)

```bash
go run .
```

### Web UI

```bash
go run . -web                    # serves http://localhost:8080
go run . -web -addr :9000        # listen on all interfaces, port 9000
```

Open the printed URL in a browser. The web UI lets you:

- **Play / pause / step** the simulation (single step, +10, +100) and adjust the speed (10 ms - 1 s per generation).
- **Draw cells** by clicking or dragging on the board.
- **Randomize** the board with an adjustable density, or **clear** it.
- **Resize** the board (5-200 cells per side; existing cells are kept).
- **Load classic patterns** (glider, blinker, toad, beacon, pulsar, lightweight spaceship, R-pentomino, diehard, acorn, Gosper glider gun): either centered on a cleared board, or stamped wherever you click (hold Shift to stamp repeatedly).
- **Import / export** boards as plain text (`#` = alive, `.` = dead).
- See the live **generation** and **population** counts. The simulation pauses itself when the board stops changing.
- Keyboard shortcuts: <kbd>Space</kbd> play/pause, <kbd>N</kbd> step, <kbd>R</kbd> randomize, <kbd>C</kbd> clear, <kbd>Esc</kbd> cancel stamping.

The simulation runs on the server. Every open browser tab gets live updates over Server-Sent Events, so all tabs show the same board. The HTML/CSS/JS assets are embedded in the binary (`go build` gives you one self-contained executable), and it uses only the standard library.

#### HTTP API

Every `POST` endpoint returns the new game state as JSON (`width`, `height`, `generation`, `population`, `running`, `speedMs`, `rows`). Request bodies are optional unless noted.

| Method | Path              | Body                                             |
|--------|-------------------|--------------------------------------------------|
| GET    | `/api/state`      | -                                                |
| GET    | `/api/patterns`   | -                                                |
| GET    | `/api/events`     | - (Server-Sent Events stream of `state`)         |
| POST   | `/api/play`       | -                                                |
| POST   | `/api/pause`      | -                                                |
| POST   | `/api/step`       | `{"count": 1}` (1-1000)                          |
| POST   | `/api/speed`      | `{"speedMs": 100}` (10-2000), required           |
| POST   | `/api/randomize`  | `{"density": 0.25}`                              |
| POST   | `/api/clear`      | -                                                |
| POST   | `/api/cells`      | `{"cells": [{"x": 1, "y": 2, "alive": true}]}`   |
| POST   | `/api/resize`     | `{"width": 80, "height": 40}`, required          |
| POST   | `/api/pattern`    | `{"name": "glider", "x": 0, "y": 0, "clear": false}` (centered if x/y omitted) |
| POST   | `/api/load`       | `{"rows": [".#.", "..#", "###", ...]}`           |

### Tests

```bash
go test ./...
```

### Configuration

The following constants at the top of `main.go` can be tuned:

| Constant   | Default | Description                              |
|------------|---------|------------------------------------------|
| `width`    | 60      | Number of columns in the grid            |
| `height`   | 30      | Number of rows in the grid               |
| `liveChar` | "█"     | Character used to draw live cells        |
| `frameRate`| 100 ms  | Delay between successive frames          |

## Example frame

```
                    ████                    
                  █     █                   
                 █       █                  
                  █     █                   
                    ████                    
```