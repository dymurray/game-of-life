package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

//go:embed web
var webFS embed.FS

// Speed limits (milliseconds between generations) for the web UI.
const (
	minSpeedMS     = 10
	maxSpeedMS     = 2000
	defaultSpeedMS = 100
	maxBodyBytes   = 1 << 20
	maxStepCount   = 1000
)

// game is the shared, concurrency-safe simulation state behind the web UI.
type game struct {
	mu         sync.Mutex
	g          grid
	generation int
	running    bool
	speed      time.Duration
	version    uint64 // incremented on every change
	rnd        *rand.Rand

	subMu sync.Mutex
	subs  map[chan struct{}]struct{}

	wake chan struct{} // signals the run loop that running/speed changed
	stop chan struct{}
}

// state is the JSON representation of the game sent to clients.
type state struct {
	Width      int      `json:"width"`
	Height     int      `json:"height"`
	Generation int      `json:"generation"`
	Population int      `json:"population"`
	Running    bool     `json:"running"`
	SpeedMS    int      `json:"speedMs"`
	Version    uint64   `json:"version"`
	Rows       []string `json:"rows"`
}

func newGame(w, h int, seed int64) *game {
	gm := &game{
		g:     newGridSize(w, h),
		speed: defaultSpeedMS * time.Millisecond,
		rnd:   rand.New(rand.NewSource(seed)),
		subs:  make(map[chan struct{}]struct{}),
		wake:  make(chan struct{}, 1),
		stop:  make(chan struct{}),
	}
	gm.g.seedDensity(liveChars, gm.rnd.Float64)
	return gm
}

// snapshotLocked builds a state; gm.mu must be held.
func (gm *game) snapshotLocked() state {
	return state{
		Width:      gm.g.w(),
		Height:     gm.g.h(),
		Generation: gm.generation,
		Population: gm.g.population(),
		Running:    gm.running,
		SpeedMS:    int(gm.speed / time.Millisecond),
		Version:    gm.version,
		Rows:       gm.g.rows(),
	}
}

func (gm *game) snapshot() state {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	return gm.snapshotLocked()
}

// update runs fn with the lock held, bumps the version and notifies
// subscribers. It returns the resulting state or fn's error.
func (gm *game) update(fn func() error) (state, error) {
	gm.mu.Lock()
	if err := fn(); err != nil {
		gm.mu.Unlock()
		return state{}, err
	}
	gm.version++
	s := gm.snapshotLocked()
	gm.mu.Unlock()
	gm.notify()
	return s, nil
}

func (gm *game) poke() {
	select {
	case gm.wake <- struct{}{}:
	default:
	}
}

func (gm *game) subscribe() chan struct{} {
	ch := make(chan struct{}, 1)
	gm.subMu.Lock()
	gm.subs[ch] = struct{}{}
	gm.subMu.Unlock()
	return ch
}

func (gm *game) unsubscribe(ch chan struct{}) {
	gm.subMu.Lock()
	delete(gm.subs, ch)
	gm.subMu.Unlock()
}

func (gm *game) notify() {
	gm.subMu.Lock()
	defer gm.subMu.Unlock()
	for ch := range gm.subs {
		select {
		case ch <- struct{}{}:
		default: // subscriber already has a pending notification
		}
	}
}

// run advances the simulation while it is marked running. It returns when
// gm.stop is closed.
func (gm *game) run() {
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	for {
		gm.mu.Lock()
		running, speed := gm.running, gm.speed
		gm.mu.Unlock()
		if running {
			timer.Reset(speed)
		}
		select {
		case <-gm.stop:
			timer.Stop()
			return
		case <-gm.wake:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		case <-timer.C:
			gm.update(func() error {
				if gm.running {
					gm.advanceLocked(1)
				}
				return nil
			})
		}
	}
}

// advanceLocked steps the board n generations; gm.mu must be held. If the
// board stops changing the simulation is paused automatically.
func (gm *game) advanceLocked(n int) {
	for i := 0; i < n; i++ {
		next := gm.g.step()
		if next.equal(gm.g) {
			gm.running = false
			return
		}
		gm.g = next
		gm.generation++
	}
}

// ---------------------------------------------------------------- HTTP ---

type apiError struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func decode(w http.ResponseWriter, r *http.Request, v any) error {
	if r.ContentLength == 0 {
		return nil // all request bodies are optional
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	return nil
}

var errBadRequest = errors.New("bad request")

type badRequest string

func (b badRequest) Error() string { return string(b) }
func (b badRequest) Unwrap() error { return errBadRequest }

// action wraps a POST endpoint that decodes a body of type T, mutates the
// game, and replies with the new state.
func action[T any](gm *game, fn func(req *T) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeJSON(w, http.StatusMethodNotAllowed, apiError{"method not allowed"})
			return
		}
		var req T
		if err := decode(w, r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, apiError{err.Error()})
			return
		}
		s, err := gm.update(func() error { return fn(&req) })
		if err != nil {
			code := http.StatusInternalServerError
			if errors.Is(err, errBadRequest) {
				code = http.StatusBadRequest
			}
			writeJSON(w, code, apiError{err.Error()})
			return
		}
		gm.poke()
		writeJSON(w, http.StatusOK, s)
	}
}

func checkSize(w, h int) error {
	if w < minSize || w > maxSize || h < minSize || h > maxSize {
		return badRequest(fmt.Sprintf("width and height must be between %d and %d", minSize, maxSize))
	}
	return nil
}

// newHandler returns the HTTP handler serving the UI and JSON API.
func newHandler(gm *game) http.Handler {
	mux := http.NewServeMux()

	static, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", securityHeaders(http.FileServer(http.FS(static))))

	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			writeJSON(w, http.StatusMethodNotAllowed, apiError{"method not allowed"})
			return
		}
		writeJSON(w, http.StatusOK, gm.snapshot())
	})

	mux.HandleFunc("/api/patterns", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, patterns)
	})

	mux.HandleFunc("/api/events", gm.serveEvents)

	mux.Handle("/api/play", action(gm, func(*struct{}) error {
		gm.running = true
		return nil
	}))
	mux.Handle("/api/pause", action(gm, func(*struct{}) error {
		gm.running = false
		return nil
	}))

	type stepReq struct {
		Count int `json:"count"`
	}
	mux.Handle("/api/step", action(gm, func(req *stepReq) error {
		if req.Count == 0 {
			req.Count = 1
		}
		if req.Count < 1 || req.Count > maxStepCount {
			return badRequest(fmt.Sprintf("count must be between 1 and %d", maxStepCount))
		}
		gm.running = false
		gm.advanceLocked(req.Count)
		return nil
	}))

	type speedReq struct {
		SpeedMS int `json:"speedMs"`
	}
	mux.Handle("/api/speed", action(gm, func(req *speedReq) error {
		if req.SpeedMS < minSpeedMS || req.SpeedMS > maxSpeedMS {
			return badRequest(fmt.Sprintf("speedMs must be between %d and %d", minSpeedMS, maxSpeedMS))
		}
		gm.speed = time.Duration(req.SpeedMS) * time.Millisecond
		return nil
	}))

	type randomReq struct {
		Density *float64 `json:"density"`
	}
	mux.Handle("/api/randomize", action(gm, func(req *randomReq) error {
		d := liveChars
		if req.Density != nil {
			d = *req.Density
		}
		if d < 0 || d > 1 {
			return badRequest("density must be between 0 and 1")
		}
		gm.g.seedDensity(d, gm.rnd.Float64)
		gm.generation = 0
		return nil
	}))

	mux.Handle("/api/clear", action(gm, func(*struct{}) error {
		gm.g = newGridSize(gm.g.w(), gm.g.h())
		gm.generation = 0
		gm.running = false
		return nil
	}))

	type cell struct {
		X     int  `json:"x"`
		Y     int  `json:"y"`
		Alive bool `json:"alive"`
	}
	type cellsReq struct {
		Cells []cell `json:"cells"`
	}
	mux.Handle("/api/cells", action(gm, func(req *cellsReq) error {
		w, h := gm.g.w(), gm.g.h()
		for _, c := range req.Cells {
			if c.X < 0 || c.X >= w || c.Y < 0 || c.Y >= h {
				return badRequest(fmt.Sprintf("cell (%d,%d) is outside the %dx%d board", c.X, c.Y, w, h))
			}
		}
		for _, c := range req.Cells {
			gm.g[c.Y][c.X] = c.Alive
		}
		return nil
	}))

	type resizeReq struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	}
	mux.Handle("/api/resize", action(gm, func(req *resizeReq) error {
		if err := checkSize(req.Width, req.Height); err != nil {
			return err
		}
		gm.g = gm.g.resized(req.Width, req.Height)
		return nil
	}))

	type patternReq struct {
		Name  string `json:"name"`
		X     *int   `json:"x"`
		Y     *int   `json:"y"`
		Clear bool   `json:"clear"`
	}
	mux.Handle("/api/pattern", action(gm, func(req *patternReq) error {
		p, ok := findPattern(req.Name)
		if !ok {
			return badRequest(fmt.Sprintf("unknown pattern %q", req.Name))
		}
		pw, ph := p.size()
		if pw > gm.g.w() || ph > gm.g.h() {
			return badRequest(fmt.Sprintf("pattern %s (%dx%d) does not fit on a %dx%d board", p.Label, pw, ph, gm.g.w(), gm.g.h()))
		}
		if req.Clear {
			gm.g = newGridSize(gm.g.w(), gm.g.h())
			gm.generation = 0
		}
		// Center the pattern unless an explicit position was given.
		x, y := (gm.g.w()-pw)/2, (gm.g.h()-ph)/2
		if req.X != nil {
			x = *req.X
		}
		if req.Y != nil {
			y = *req.Y
		}
		gm.g.place(p, x, y)
		return nil
	}))

	type loadReq struct {
		Rows []string `json:"rows"`
	}
	mux.Handle("/api/load", action(gm, func(req *loadReq) error {
		g, err := parseRows(req.Rows)
		if err != nil {
			return badRequest(err.Error())
		}
		gm.g = g
		gm.generation = 0
		gm.running = false
		return nil
	}))

	return mux
}

// serveEvents streams the game state as Server-Sent Events whenever it
// changes.
func (gm *game) serveEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, apiError{"streaming unsupported"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")

	ch := gm.subscribe()
	defer gm.unsubscribe(ch)

	send := func() error {
		b, err := json.Marshal(gm.snapshot())
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "event: state\ndata: %s\n\n", b); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}
	if err := send(); err != nil {
		return
	}
	keepAlive := time.NewTicker(25 * time.Second)
	defer keepAlive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-gm.stop:
			return
		case <-keepAlive.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-ch:
			if err := send(); err != nil {
				return
			}
		}
	}
}

func securityHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; img-src 'self' data:")
		h.ServeHTTP(w, r)
	})
}

// displayAddr turns a listen address into something clickable in a log line.
func displayAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return host + ":" + port
}

// serve starts the web UI on addr and blocks until the server fails.
func serve(addr string) error {
	gm := newGame(width, height, time.Now().UnixNano())
	go gm.run()
	defer close(gm.stop)

	srv := &http.Server{
		Addr:              addr,
		Handler:           newHandler(gm),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	return srv.ListenAndServe()
}
