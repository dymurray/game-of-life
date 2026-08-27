# Conway's Game of Life (Go)

A minimal terminal implementation of Conway's Game of Life written in Go.

## What is Conway's Game of Life?

Conway's Game of Life is a zero-player cellular automaton devised by mathematician John Horton Conway in 1970. The "game" consists of a two-dimensional orthogonal grid of square cells, each of which is in one of two possible states: alive or dead. Every cell interacts with its eight neighbours (the Moore neighbourhood) and the following four rules are applied simultaneously to every cell in each generation:

- **Underpopulation**: Any live cell with fewer than two live neighbours dies.
- **Survival**: Any live cell with two or three live neighbours survives to the next generation.
- **Overpopulation**: Any live cell with more than three live neighbours dies.
- **Reproduction**: Any dead cell with exactly three live neighbours becomes a live cell.

The game evolves from an initial configuration according to these rules; common emergent patterns include still lifes (stable shapes), oscillators (e.g. the blinker), and spaceships (e.g. the glider).

## How this program works

- Written in pure Go using only the standard library (`math/rand`, `os`, `strings`, `time`).
- The board is represented as a `grid` (`[][]bool`) with a default size of **60×30**, randomly seeded so that approximately 25 % of cells start alive.
- **Toroidal topology**: neighbour counting (`countNeighbors`) wraps around the edges using modulo arithmetic, so patterns that leave one side of the grid reappear on the opposite side.
- `step` computes the next generation into a fresh grid by applying Conway's four rules.
- `render` clears the terminal with the ANSI escape sequence `\033[H\033[2J` and redraws the grid in place, producing an animation running at roughly 10 frames per second.
- The program loops forever until the user presses **Ctrl+C**.

## Usage

### Prerequisites

- Go 1.21 or later.

### Run

```bash
go run main.go
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