package main

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStepBlinker(t *testing.T) {
	g := newGridSize(5, 5)
	g.place(pattern{Cells: []string{"###"}}, 1, 2)
	g2 := g.step()
	want, _ := parseRows([]string{".....", "..#..", "..#..", "..#..", "....."})
	if !g2.equal(want) {
		t.Fatalf("got %v", g2.rows())
	}
	if !g2.step().equal(g) {
		t.Fatal("blinker should have period 2")
	}
}

func TestToroidalWrap(t *testing.T) {
	g := newGridSize(6, 5)
	g[0][0] = true
	if n := g.countNeighbors(5, 4); n != 1 {
		t.Fatalf("corner neighbors = %d, want 1", n)
	}
}

func TestParseRowsErrors(t *testing.T) {
	for _, rows := range [][]string{
		{"#####"},
		{".....", ".....", ".....", ".....", "...."},
		{".....", ".....", ".....", ".....", "..x.."},
	} {
		if _, err := parseRows(rows); err == nil {
			t.Errorf("expected error for %v", rows)
		}
	}
}

func TestPatternsFitDefaultBoard(t *testing.T) {
	for _, p := range patterns {
		w, h := p.size()
		if w > width || h > height {
			t.Errorf("%s too large", p.Name)
		}
		for _, r := range p.Cells {
			if strings.Trim(r, "#.") != "" {
				t.Errorf("%s has invalid chars", p.Name)
			}
		}
	}
}

func setup(t *testing.T) (*game, *httptest.Server) {
	gm := newGame(20, 10, 1)
	go gm.run()
	srv := httptest.NewServer(newHandler(gm))
	t.Cleanup(func() { srv.Close(); close(gm.stop) })
	return gm, srv
}

func post(t *testing.T, srv *httptest.Server, path, body string) (int, state) {
	t.Helper()
	res, err := http.Post(srv.URL+"/api/"+path, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var s state
	json.NewDecoder(res.Body).Decode(&s)
	return res.StatusCode, s
}

func TestAPI(t *testing.T) {
	_, srv := setup(t)

	if code, s := post(t, srv, "clear", ""); code != 200 || s.Population != 0 || s.Generation != 0 {
		t.Fatalf("clear: %d %+v", code, s)
	}
	if _, s := post(t, srv, "cells", `{"cells":[{"x":1,"y":1,"alive":true},{"x":2,"y":1,"alive":true},{"x":3,"y":1,"alive":true}]}`); s.Population != 3 {
		t.Fatalf("cells: %+v", s)
	}
	if _, s := post(t, srv, "step", ""); s.Generation != 1 || s.Rows[0][2] != '#' || s.Rows[1][1] != '.' {
		t.Fatalf("step: %+v", s)
	}
	if _, s := post(t, srv, "step", `{"count":3}`); s.Generation != 4 {
		t.Fatalf("step 3: %+v", s)
	}
	if _, s := post(t, srv, "resize", `{"width":30,"height":15}`); s.Width != 30 || s.Height != 15 || s.Population != 3 {
		t.Fatalf("resize: %+v", s)
	}
	if _, s := post(t, srv, "pattern", `{"name":"glider","clear":true}`); s.Population != 5 {
		t.Fatalf("pattern: %+v", s)
	}
	if _, s := post(t, srv, "speed", `{"speedMs":250}`); s.SpeedMS != 250 {
		t.Fatalf("speed: %+v", s)
	}
	if _, s := post(t, srv, "randomize", `{"density":1}`); s.Population != 30*15 {
		t.Fatalf("randomize: %+v", s)
	}
	if _, s := post(t, srv, "load", `{"rows":[".....","..#..","..#..","..#..","....."]}`); s.Width != 5 || s.Population != 3 {
		t.Fatalf("load: %+v", s)
	}

	bad := map[string]string{
		"cells":     `{"cells":[{"x":99,"y":0,"alive":true}]}`,
		"resize":    `{"width":1,"height":1000}`,
		"pattern":   `{"name":"nope"}`,
		"speed":     `{"speedMs":1}`,
		"step":      `{"count":100000}`,
		"randomize": `{"density":2}`,
		"load":      `{"rows":["#"]}`,
		"play":      `{"bogus":1}`,
	}
	for path, body := range bad {
		if code, _ := post(t, srv, path, body); code != 400 {
			t.Errorf("%s %s: got %d, want 400", path, body, code)
		}
	}

	res, _ := http.Get(srv.URL + "/api/play")
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/play = %d", res.StatusCode)
	}
}

func TestPlayAdvancesAndAutoPauses(t *testing.T) {
	_, srv := setup(t)
	post(t, srv, "pattern", `{"name":"glider","clear":true}`)
	post(t, srv, "speed", `{"speedMs":10}`)
	post(t, srv, "play", "")
	deadline := time.Now().Add(2 * time.Second)
	for {
		res, _ := http.Get(srv.URL + "/api/state")
		var s state
		json.NewDecoder(res.Body).Decode(&s)
		res.Body.Close()
		if s.Generation >= 5 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("did not advance: %+v", s)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, s := post(t, srv, "pause", ""); s.Running {
		t.Fatal("still running")
	}

	// A still life should pause itself.
	post(t, srv, "load", `{"rows":[".....",".##..",".##..",".....","....."]}`)
	post(t, srv, "play", "")
	time.Sleep(100 * time.Millisecond)
	res, _ := http.Get(srv.URL + "/api/state")
	var s state
	json.NewDecoder(res.Body).Decode(&s)
	res.Body.Close()
	if s.Running {
		t.Fatal("still life should auto-pause")
	}
}

func TestStaticAndEvents(t *testing.T) {
	_, srv := setup(t)
	for _, p := range []string{"/", "/app.js", "/style.css"} {
		res, err := http.Get(srv.URL + p)
		if err != nil || res.StatusCode != 200 {
			t.Fatalf("GET %s: %v %v", p, err, res)
		}
		if res.Header.Get("Content-Security-Policy") == "" {
			t.Errorf("%s missing CSP", p)
		}
		res.Body.Close()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/events", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	sc := bufio.NewScanner(res.Body)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	seen := 0
	go func() {
		r, err := http.Post(srv.URL+"/api/step", "application/json", nil)
		if err == nil {
			r.Body.Close()
		}
	}()
	for sc.Scan() && seen < 2 {
		if strings.HasPrefix(sc.Text(), "data: ") {
			seen++
		}
	}
	if seen < 2 {
		t.Fatalf("got %d events", seen)
	}
}
