// Command game-of-life is an implementation of Conway's Game of Life.
//
// By default the board is seeded randomly and evolves continuously in the
// terminal on a toroidal grid (the edges wrap around). Each generation is
// rendered in place until the program is interrupted with Ctrl+C.
//
// With -web, an interactive browser UI is served instead:
//
//	go run . -web -addr :8080
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

const (
	width     = 60                     // number of columns
	height    = 30                     // number of rows
	liveChars = 0.25                   // initial fraction of live cells
	frameRate = 100 * time.Millisecond // delay between generations
)

// render draws the board to w, clearing the screen first so successive
// frames animate in place.
func (g grid) render(w io.Writer) {
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
	io.WriteString(w, b.String())
}

func showSplash() {
	fmt.Print("\033[H\033[2J") // clear
	fmt.Println("   Conway's Game of Life")
	fmt.Println()
	fmt.Println("   A zero-player cellular automaton.")
	fmt.Println("   Starting simulation...")
	time.Sleep(1500 * time.Millisecond)
}

func runTerminal() {
	showSplash()

	g := newGrid()
	g.seed()

	for {
		g.render(os.Stdout)
		time.Sleep(frameRate)
		g = g.step()
	}
}

func main() {
	web := flag.Bool("web", false, "serve the interactive web UI instead of running in the terminal")
	addr := flag.String("addr", "localhost:8080", "listen address for the web UI (with -web)")
	flag.Parse()

	if !*web {
		runTerminal()
		return
	}

	log.Printf("Game of Life web UI listening on http://%s", displayAddr(*addr))
	if err := serve(*addr); err != nil {
		log.Fatal(err)
	}
}
