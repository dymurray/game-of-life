# Conway's Game of Life (Go)

A minimal terminal implementation of Conway's Game of Life written in Go.

## What is Conway's Game of Life?

A zero-player cellular automaton on a 2D grid where each cell is alive or dead. Every generation the following four rules are applied simultaneously:

- **Underpopulation**: live cell with <2 live neighbours dies.
- **Survival**: live cell with 2–3 live neighbours survives.
- **Overpopulation**: live cell with >3 live neighbours dies.
- **Reproduction**: dead cell with exactly 3 live neighbours becomes alive.

## How this program works

- Pure Go (std lib only).
- 60×30 toroidal grid, ~25 % cells start alive.
- `step` applies the rules; `render` draws via ANSI escapes (~10 fps).

## Usage

```bash
go run main.go
```

### Configuration (`main.go`)

| Constant   | Default | Description                     |
|------------|---------|---------------------------------|
| `width`    | 60      | Columns                         |
| `height`   | 30      | Rows                            |
| `liveChar` | "█"     | Live-cell glyph                 |
| `frameRate`| 100 ms  | Frame delay                     |