package main

import (
	"fmt"
	"math/rand"
	"strings"
)

// Limits on board dimensions accepted from users (e.g. the web UI).
const (
	minSize = 5
	maxSize = 200
)

// grid holds the state of every cell: true means alive. Rows are indexed
// first (g[y][x]). All rows have the same length.
type grid [][]bool

// newGridSize allocates an empty h x w board.
func newGridSize(w, h int) grid {
	g := make(grid, h)
	for y := range g {
		g[y] = make([]bool, w)
	}
	return g
}

// newGrid allocates an empty board using the default terminal dimensions.
func newGrid() grid {
	return newGridSize(width, height)
}

// w returns the number of columns of g.
func (g grid) w() int {
	if len(g) == 0 {
		return 0
	}
	return len(g[0])
}

// h returns the number of rows of g.
func (g grid) h() int { return len(g) }

// seed randomly populates the board with live cells at the default density.
func (g grid) seed() {
	g.seedDensity(liveChars, rand.Float64)
}

// seedDensity populates the board so that each cell is alive with
// probability density, using rnd as the source of randomness.
func (g grid) seedDensity(density float64, rnd func() float64) {
	for y := range g {
		for x := range g[y] {
			g[y][x] = rnd() < density
		}
	}
}

// countNeighbors returns the number of live cells surrounding (x, y),
// wrapping around the edges so the grid behaves as a torus.
func (g grid) countNeighbors(x, y int) int {
	w, h := g.w(), g.h()
	count := 0
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			nx := (x + dx + w) % w
			ny := (y + dy + h) % h
			if g[ny][nx] {
				count++
			}
		}
	}
	return count
}

// step returns the next generation by applying Conway's rules to every cell.
func (g grid) step() grid {
	next := newGridSize(g.w(), g.h())
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

// population returns the number of live cells.
func (g grid) population() int {
	n := 0
	for y := range g {
		for x := range g[y] {
			if g[y][x] {
				n++
			}
		}
	}
	return n
}

// clone returns a deep copy of g.
func (g grid) clone() grid {
	c := newGridSize(g.w(), g.h())
	for y := range g {
		copy(c[y], g[y])
	}
	return c
}

// resized returns a copy of g with the new dimensions, preserving the
// overlapping top-left region.
func (g grid) resized(w, h int) grid {
	c := newGridSize(w, h)
	for y := 0; y < h && y < g.h(); y++ {
		for x := 0; x < w && x < g.w(); x++ {
			c[y][x] = g[y][x]
		}
	}
	return c
}

// equal reports whether g and o have identical dimensions and cells.
func (g grid) equal(o grid) bool {
	if g.h() != o.h() || g.w() != o.w() {
		return false
	}
	for y := range g {
		for x := range g[y] {
			if g[y][x] != o[y][x] {
				return false
			}
		}
	}
	return true
}

// rows encodes g as one string per row using '#' for live and '.' for dead.
func (g grid) rows() []string {
	out := make([]string, g.h())
	var b strings.Builder
	for y := range g {
		b.Reset()
		for x := range g[y] {
			if g[y][x] {
				b.WriteByte('#')
			} else {
				b.WriteByte('.')
			}
		}
		out[y] = b.String()
	}
	return out
}

// parseRows decodes the format produced by rows. Any of '#', 'O', 'o', '*',
// '1' mark a live cell; '.', ' ', '0', '_' mark a dead cell.
func parseRows(rows []string) (grid, error) {
	if len(rows) < minSize || len(rows) > maxSize {
		return nil, fmt.Errorf("height must be between %d and %d", minSize, maxSize)
	}
	w := len(rows[0])
	if w < minSize || w > maxSize {
		return nil, fmt.Errorf("width must be between %d and %d", minSize, maxSize)
	}
	g := newGridSize(w, len(rows))
	for y, r := range rows {
		if len(r) != w {
			return nil, fmt.Errorf("row %d has length %d, want %d", y, len(r), w)
		}
		for x := 0; x < w; x++ {
			switch r[x] {
			case '#', 'O', 'o', '*', '1':
				g[y][x] = true
			case '.', ' ', '0', '_':
			default:
				return nil, fmt.Errorf("invalid character %q at row %d col %d", r[x], y, x)
			}
		}
	}
	return g, nil
}

// pattern is a named, well-known starting configuration.
type pattern struct {
	Name  string   `json:"name"`
	Label string   `json:"label"`
	Cells []string `json:"cells"`
}

// patterns lists the shapes available from the web UI.
var patterns = []pattern{
	{"glider", "Glider", []string{".#.", "..#", "###"}},
	{"blinker", "Blinker", []string{"###"}},
	{"toad", "Toad", []string{".###", "###."}},
	{"beacon", "Beacon", []string{"##..", "##..", "..##", "..##"}},
	{"pulsar", "Pulsar", []string{
		"..###...###..",
		".............",
		"#....#.#....#",
		"#....#.#....#",
		"#....#.#....#",
		"..###...###..",
		".............",
		"..###...###..",
		"#....#.#....#",
		"#....#.#....#",
		"#....#.#....#",
		".............",
		"..###...###..",
	}},
	{"lwss", "Lightweight spaceship", []string{".#..#", "#....", "#...#", "####."}},
	{"rpentomino", "R-pentomino", []string{".##", "##.", ".#."}},
	{"diehard", "Diehard", []string{"......#.", "##......", ".#...###"}},
	{"acorn", "Acorn", []string{".#.....", "...#...", "##..###"}},
	{"gosper", "Gosper glider gun", []string{
		"........................#...........",
		"......................#.#...........",
		"............##......##............##",
		"...........#...#....##............##",
		"##........#.....#...##..............",
		"##........#...#.##....#.#...........",
		"..........#.....#.......#...........",
		"...........#...#....................",
		"............##......................",
	}},
}

// findPattern looks up a pattern by name.
func findPattern(name string) (pattern, bool) {
	for _, p := range patterns {
		if p.Name == name {
			return p, true
		}
	}
	return pattern{}, false
}

// place stamps p onto g with its top-left corner at (x, y), wrapping around
// the edges. Only live cells of the pattern are written.
func (g grid) place(p pattern, x, y int) {
	w, h := g.w(), g.h()
	for dy, row := range p.Cells {
		for dx := 0; dx < len(row); dx++ {
			if row[dx] == '#' {
				g[((y+dy)%h+h)%h][((x+dx)%w+w)%w] = true
			}
		}
	}
}

// size returns the bounding box of p.
func (p pattern) size() (w, h int) {
	for _, r := range p.Cells {
		if len(r) > w {
			w = len(r)
		}
	}
	return w, len(p.Cells)
}
