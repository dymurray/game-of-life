// Web UI for Conway's Game of Life. All state lives on the server; this
// script renders it and sends user actions to the JSON API.
(() => {
  "use strict";

  const $ = (id) => document.getElementById(id);
  const canvas = $("board");
  const ctx = canvas.getContext("2d");

  let st = null;            // last state from the server
  let cells = [];           // cells[y][x] booleans (local copy, incl. pending edits)
  let pending = new Map();  // "x,y" -> alive: edits drawn but not yet acknowledged
  let cellSize = 12;
  let drawing = null;       // {alive: bool} while dragging
  let lastCell = null;
  let stampMode = false;
  let hover = null;         // {x, y} cell under the pointer
  let patterns = [];
  let flushTimer = null;

  // ------------------------------------------------------------ API ---
  async function api(path, body) {
    hideError();
    try {
      const res = await fetch("/api/" + path, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: body === undefined ? "" : JSON.stringify(body),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || res.statusText);
      applyState(data);
      return data;
    } catch (err) {
      showError(err.message);
      throw err;
    }
  }

  function showError(msg) {
    const el = $("error");
    el.textContent = msg;
    el.hidden = false;
    clearTimeout(showError.t);
    showError.t = setTimeout(hideError, 6000);
  }
  function hideError() { $("error").hidden = true; }

  // ---------------------------------------------------------- state ---
  function applyState(s) {
    if (st && s.version < st.version) return; // stale
    const resized = !st || st.width !== s.width || st.height !== s.height;
    st = s;
    cells = s.rows.map((r) => Array.from(r, (c) => c === "#"));
    if (resized) {
      pending.clear();
      $("width").value = s.width;
      $("height").value = s.height;
      layout();
    }
    // Re-apply edits that the server hasn't seen yet so drawing feels instant.
    for (const [k, alive] of pending) {
      const [x, y] = k.split(",").map(Number);
      if (cells[y] && x < s.width) cells[y][x] = alive;
    }
    $("generation").textContent = s.generation.toLocaleString();
    $("population").textContent = s.population.toLocaleString();
    $("dims").textContent = `${s.width}×${s.height}`;
    const play = $("play");
    play.textContent = s.running ? "❚❚ Pause" : "▶ Play";
    play.classList.toggle("running", s.running);
    if (document.activeElement !== $("speed")) {
      $("speed").value = s.speedMs;
      $("speedOut").textContent = s.speedMs + " ms";
    }
    draw();
  }

  // --------------------------------------------------------- render ---
  function layout() {
    if (!st) return;
    const wrap = canvas.parentElement;
    const availW = wrap.clientWidth - 4;
    const availH = Math.max(200, window.innerHeight - canvas.getBoundingClientRect().top - 60);
    cellSize = Math.max(2, Math.floor(Math.min(availW / st.width, availH / st.height)));
    const dpr = window.devicePixelRatio || 1;
    const w = cellSize * st.width, h = cellSize * st.height;
    canvas.style.width = w + "px";
    canvas.style.height = h + "px";
    canvas.width = Math.round(w * dpr);
    canvas.height = Math.round(h * dpr);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    draw();
  }

  function draw() {
    if (!st) return;
    const w = st.width, h = st.height, s = cellSize;
    ctx.fillStyle = "#000";
    ctx.fillRect(0, 0, w * s, h * s);

    ctx.fillStyle = "#3fb950";
    const gap = $("gridlines").checked && s >= 5 ? 1 : 0;
    for (let y = 0; y < h; y++) {
      const row = cells[y];
      for (let x = 0; x < w; x++) {
        if (row[x]) ctx.fillRect(x * s, y * s, s - gap, s - gap);
      }
    }

    if (gap) {
      ctx.strokeStyle = "rgba(255,255,255,0.06)";
      ctx.lineWidth = 1;
      ctx.beginPath();
      for (let x = 0; x <= w; x++) { ctx.moveTo(x * s - 0.5, 0); ctx.lineTo(x * s - 0.5, h * s); }
      for (let y = 0; y <= h; y++) { ctx.moveTo(0, y * s - 0.5); ctx.lineTo(w * s, y * s - 0.5); }
      ctx.stroke();
    }

    // Stamp preview.
    if (stampMode && hover) {
      const p = currentPattern();
      if (p) {
        ctx.fillStyle = "rgba(88,166,255,0.55)";
        p.cells.forEach((row, dy) => {
          for (let dx = 0; dx < row.length; dx++) {
            if (row[dx] !== "#") continue;
            const x = (hover.x + dx) % w, y = (hover.y + dy) % h;
            ctx.fillRect(x * s, y * s, s - gap, s - gap);
          }
        });
      }
    } else if (hover && !drawing) {
      ctx.strokeStyle = "rgba(255,255,255,0.5)";
      ctx.strokeRect(hover.x * s + 0.5, hover.y * s + 0.5, s - 1, s - 1);
    }
  }

  // ---------------------------------------------------------- input ---
  function cellAt(ev) {
    const r = canvas.getBoundingClientRect();
    const x = Math.floor((ev.clientX - r.left) / cellSize);
    const y = Math.floor((ev.clientY - r.top) / cellSize);
    if (!st || x < 0 || y < 0 || x >= st.width || y >= st.height) return null;
    return { x, y };
  }

  function paint(c) {
    if (!c || cells[c.y][c.x] === drawing.alive) return;
    cells[c.y][c.x] = drawing.alive;
    pending.set(c.x + "," + c.y, drawing.alive);
    scheduleFlush();
  }

  // Bresenham so fast drags don't leave gaps.
  function paintLine(a, b) {
    let { x: x0, y: y0 } = a;
    const { x: x1, y: y1 } = b;
    const dx = Math.abs(x1 - x0), dy = -Math.abs(y1 - y0);
    const sx = x0 < x1 ? 1 : -1, sy = y0 < y1 ? 1 : -1;
    let err = dx + dy;
    for (;;) {
      paint({ x: x0, y: y0 });
      if (x0 === x1 && y0 === y1) break;
      const e2 = 2 * err;
      if (e2 >= dy) { err += dy; x0 += sx; }
      if (e2 <= dx) { err += dx; y0 += sy; }
    }
  }

  function scheduleFlush() {
    if (flushTimer) return;
    flushTimer = setTimeout(flush, 50);
  }

  async function flush() {
    flushTimer = null;
    if (!pending.size) return;
    const batch = [...pending].map(([k, alive]) => {
      const [x, y] = k.split(",").map(Number);
      return { x, y, alive };
    });
    const sent = new Map(pending);
    try {
      await api("cells", { cells: batch });
    } catch (_) { /* error shown already */ }
    // Drop acknowledged edits unless they've been changed again since.
    for (const [k, v] of sent) if (pending.get(k) === v) pending.delete(k);
    draw();
  }

  canvas.addEventListener("pointerdown", (ev) => {
    const c = cellAt(ev);
    if (!c) return;
    canvas.focus();
    if (stampMode) {
      const p = currentPattern();
      if (p) api("pattern", { name: p.name, x: c.x, y: c.y }).catch(() => {});
      if (!ev.shiftKey) setStamp(false);
      return;
    }
    canvas.setPointerCapture(ev.pointerId);
    drawing = { alive: !cells[c.y][c.x] };
    lastCell = c;
    paint(c);
    draw();
  });

  canvas.addEventListener("pointermove", (ev) => {
    const c = cellAt(ev);
    hover = c;
    if (drawing && c) {
      paintLine(lastCell, c);
      lastCell = c;
    }
    draw();
  });

  const endDraw = () => {
    if (!drawing) return;
    drawing = null;
    lastCell = null;
    flush();
  };
  canvas.addEventListener("pointerup", endDraw);
  canvas.addEventListener("pointercancel", endDraw);
  canvas.addEventListener("pointerleave", () => { hover = null; draw(); });

  // ------------------------------------------------------- controls ---
  const togglePlay = () => api(st && st.running ? "pause" : "play").catch(() => {});
  $("play").onclick = togglePlay;
  $("step").onclick = () => api("step").catch(() => {});
  $("step10").onclick = () => api("step", { count: 10 }).catch(() => {});
  $("step100").onclick = () => api("step", { count: 100 }).catch(() => {});

  const randomize = () =>
    api("randomize", { density: Number($("density").value) / 100 }).catch(() => {});
  $("random").onclick = randomize;
  $("clear").onclick = () => api("clear").catch(() => {});

  $("speed").oninput = () => { $("speedOut").textContent = $("speed").value + " ms"; };
  $("speed").onchange = () => api("speed", { speedMs: Number($("speed").value) }).catch(() => {});
  $("density").oninput = () => { $("densityOut").textContent = $("density").value + "%"; };
  $("gridlines").onchange = draw;

  $("resize").onclick = () =>
    api("resize", { width: Number($("width").value), height: Number($("height").value) }).catch(() => {});

  function currentPattern() {
    return patterns.find((p) => p.name === $("pattern").value);
  }
  function setStamp(on) {
    stampMode = on;
    canvas.classList.toggle("stamping", on);
    $("placeClick").classList.toggle("active", on);
    $("hint").textContent = on
      ? "Click the board to stamp the pattern (hold Shift to stamp repeatedly, Esc to cancel)."
      : "Click or drag on the board to draw cells.";
    draw();
  }
  $("placeCenter").onclick = () => {
    const p = currentPattern();
    if (p) api("pattern", { name: p.name, clear: true }).catch(() => {});
  };
  $("placeClick").onclick = () => setStamp(!stampMode);

  $("export").onclick = () => {
    if (!st) return;
    $("io").value = cells.map((r) => r.map((c) => (c ? "#" : ".")).join("")).join("\n");
    $("io").select();
  };
  $("import").onclick = () => {
    const rows = $("io").value.split(/\r?\n/).map((r) => r.replace(/\s+$/, "")).filter((r) => r.length);
    if (!rows.length) { showError("Paste some rows of # and . first"); return; }
    const w = Math.max(...rows.map((r) => r.length));
    api("load", { rows: rows.map((r) => r.padEnd(w, ".")) }).catch(() => {});
  };

  document.addEventListener("keydown", (ev) => {
    const tag = (ev.target.tagName || "").toLowerCase();
    if (tag === "input" || tag === "textarea" || tag === "select" || ev.ctrlKey || ev.metaKey || ev.altKey) return;
    switch (ev.key) {
      case " ": ev.preventDefault(); togglePlay(); break;
      case "n": case "N": api("step").catch(() => {}); break;
      case "r": case "R": randomize(); break;
      case "c": case "C": api("clear").catch(() => {}); break;
      case "Escape": setStamp(false); break;
    }
  });

  window.addEventListener("resize", layout);

  // --------------------------------------------------------- server ---
  function connect() {
    const conn = $("conn");
    const es = new EventSource("/api/events");
    es.addEventListener("state", (ev) => applyState(JSON.parse(ev.data)));
    es.onopen = () => { conn.textContent = "live"; conn.className = "conn online"; };
    es.onerror = () => { conn.textContent = "reconnecting…"; conn.className = "conn offline"; };
  }

  async function init() {
    try {
      const [s, ps] = await Promise.all([
        fetch("/api/state").then((r) => r.json()),
        fetch("/api/patterns").then((r) => r.json()),
      ]);
      patterns = ps;
      const sel = $("pattern");
      for (const p of ps) sel.add(new Option(p.label, p.name));
      applyState(s);
    } catch (err) {
      showError("Could not reach the server: " + err.message);
    }
    connect();
  }

  init();
})();
