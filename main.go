// Command game-of-life is a terminal implementation of Conway's Game of Life.
//
// The board is seeded randomly and evolves continuously on a toroidal grid
// (the edges wrap around). Each generation is rendered in place until the
// program is interrupted with Ctrl+C.
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"
)

var (
	width     int
	height    int
	liveChars float64
	frameRate time.Duration
)

func init() {
	flag.IntVar(&width, "width", 60, "number of columns")
	flag.IntVar(&height, "height", 30, "number of rows")
	flag.Float64Var(&liveChars, "density", 0.25, "initial fraction of live cells")
	flag.DurationVar(&frameRate, "rate", 100*time.Millisecond, "delay between generations")
}

func validate() {
	if width <= 0 || height <= 0 {
		fmt.Fprintln(os.Stderr, "error: width and height must be positive")
		os.Exit(1)
	}
	if liveChars < 0 || liveChars > 1 {
		fmt.Fprintln(os.Stderr, "error: density must be between 0 and 1")
		os.Exit(1)
	}
	if frameRate <= 0 {
		fmt.Fprintln(os.Stderr, "error: rate must be positive")
		os.Exit(1)
	}
}

// grid holds the state of every cell: true means alive.
type grid [][]bool

// newGrid allocates an empty height x width board.
func newGrid() grid {
	g := make(grid, height)
	for y := range g {
		g[y] = make([]bool, width)
	}
	return g
}

// seed randomly populates the board with live cells.
func (g grid) seed() {
	for y := range g {
		for x := range g[y] {
			g[y][x] = rand.Float64() < liveChars
		}
	}
}

// countNeighbors returns the number of live cells surrounding (x, y),
// wrapping around the edges so the grid behaves as a torus.
func (g grid) countNeighbors(x, y int) int {
	count := 0
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			nx := (x + dx + width) % width
			ny := (y + dy + height) % height
			if g[ny][nx] {
				count++
			}
		}
	}
	return count
}

// step returns the next generation by applying Conway's rules to every cell.
func (g grid) step() grid {
	next := newGrid()
	for y := range g {
		for x := range g[y] {
			n := g.countNeighbors(x, y)
			if g[y][x] {
				// A live cell survives with 2 or 3 live neighbors.
				next[y][x] = n == 2 || n == 3
			} else {
				// A dead cell becomes alive with exactly 3 live neighbors.
				next[y][x] = n == 3
			}
		}
	}
	return next
}

// render draws the board to w, clearing the screen first so successive
// frames animate in place.
func (g grid) render(w *os.File) {
	var b strings.Builder
	b.WriteString("\033[H\033[2J") // cursor home + clear screen
	for y := range g {
		for x := range g[y] {
			if g[y][x] {
				b.WriteRune('█')
			} else {
				b.WriteByte(' ')
			}
		}
		b.WriteByte('\n')
	}
	w.WriteString(b.String())
}

func showSplash() {
	fmt.Print("\033[H\033[2J") // clear
	fmt.Println("   Conway's Game of Life")
	fmt.Println()
	fmt.Println("   A zero-player cellular automaton.")
	fmt.Println("   Starting simulation...")
	time.Sleep(1500 * time.Millisecond)
}

func main() {
	flag.Parse()
	validate()
	showSplash()

	g := newGrid()
	g.seed()

	for {
		g.render(os.Stdout)
		time.Sleep(frameRate)
		g = g.step()
	}
}
