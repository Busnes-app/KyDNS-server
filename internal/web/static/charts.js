// Draws the dashboard's three charts from /stats.json. No library: three
// shapes, one axis each, drawn straight into the inline SVG the page ships.
(function () {
  const NS = "http://www.w3.org/2000/svg";
  const PAD = { l: 40, r: 6, t: 8, b: 16 };
  // The queries chart also labels its time axis and names the value unit above it.
  const QPAD = { l: 40, r: 6, t: 24, b: 26 };
  const SERIES = [
    { key: "authoritative", label: "Local", color: "--series-1" },
    { key: "forwarded", label: "Forwarded", color: "--series-2" },
    { key: "blocked", label: "Blocked", color: "--series-3" },
  ];
  // Time-axis label steps in minutes; the smallest that leaves room for "23:30" wins.
  const STEPS = [5, 10, 15, 20, 30, 60, 120, 180, 240, 360, 720];

  let view = "day";
  let latest = null;

  const el = (name, attrs) => {
    const n = document.createElementNS(NS, name);
    for (const k in attrs) n.setAttribute(k, attrs[k]);
    return n;
  };
  const hue = (v) => getComputedStyle(document.documentElement).getPropertyValue(v).trim();
  const hm = (minute) => {
    const d = new Date(minute * 60000);
    return d.getHours() + ":" + String(d.getMinutes()).padStart(2, "0");
  };

  // niceMax rounds the axis ceiling up to a 1/2/5 step, so the midpoint
  // gridline lands on a round number instead of being labelled 2 at 1.5.
  function niceMax(max) {
    if (!(max > 0)) return 1;
    const pow = Math.pow(10, Math.floor(Math.log10(max)));
    for (const step of [1, 2, 5, 10]) {
      if (max <= step * pow) return step * pow;
    }
    return 10 * pow;
  }

  // merge sums buckets into clock-aligned slots of span minutes, so an hour row
  // or a wider phone bar starts on the hour rather than wherever the data did.
  function merge(buckets, span) {
    const out = [];
    for (const b of buckets) {
      const minute = Math.floor(b.minute / span) * span;
      let slot = out[out.length - 1];
      if (!slot || slot.minute !== minute) out.push(slot = { minute });
      for (const s of SERIES) slot[s.key] = (slot[s.key] || 0) + (b[s.key] || 0);
    }
    return out;
  }

  // canvas clears the svg and sizes it in CSS pixels: the viewBox follows the
  // rendered width, so text keeps its size on a phone instead of shrinking.
  function canvas(fig) {
    const svg = fig.querySelector("svg");
    const [, , w0, h] = svg.getAttribute("viewBox").split(" ").map(Number);
    const w = Math.round(svg.getBoundingClientRect().width) || w0;
    svg.setAttribute("viewBox", `0 0 ${w} ${h}`);
    while (svg.firstChild) svg.removeChild(svg.firstChild);
    svg.removeAttribute("aria-description");
    return { svg, w, h };
  }

  // empty replaces a chart that has no samples with a panel that says why,
  // instead of an axis and gridlines that suggest a flat zero.
  function empty(fig, title, hint) {
    const { svg, w, h } = canvas(fig);
    svg.appendChild(el("rect", { class: "chart-empty-bg", x: 0.5, y: 0.5, width: w - 1, height: h - 1, rx: 8 }));
    const lines = [[title, "chart-empty", h / 2 - 3], [hint, "chart-empty-hint", h / 2 + 15]];
    for (const [text, cls, y] of lines) {
      const t = el("text", { class: cls, x: w / 2, y, "text-anchor": "middle" });
      t.textContent = text;
      svg.appendChild(t);
    }
    svg.setAttribute("aria-description", title);
    const tip = fig.querySelector(".tip");
    if (tip) tip.style.display = "none";
  }

  // plot is the shared frame: it draws the y axis and its grid on a cleared
  // canvas and hands back the functions that map a sample onto it. With
  // opts.bars, sample i owns a slot and sits at its centre instead of on an edge.
  function plot(fig, max, suffix, exact, opts = {}) {
    const { svg, w, h } = canvas(fig);
    const pad = opts.pad || PAD;
    const span = opts.span || 1;
    const plotW = w - pad.l - pad.r;

    const top = exact ? max : niceMax(max);
    const x = opts.bars
      ? (i, n) => pad.l + ((i + 0.5) * plotW) / n
      : (i, n) => pad.l + (i * plotW) / Math.max(n - 1, 1);
    const at = (frac, n) => (opts.bars
      ? Math.min(n - 1, Math.floor(frac * n))
      : Math.round(frac * (n - 1)));
    const y = (v) => h - pad.b - (v / top) * (h - pad.t - pad.b);
    const head = (b) => (span > 1 ? hm(b.minute) + "–" + hm(b.minute + span) : hm(b.minute));

    // A small axis rounds its midpoint onto a neighbour — 0, 1, 1 for a 1ms
    // ceiling. Two identical labels look like a rendering fault, so the
    // midpoint is dropped rather than repeated.
    const seen = new Set();
    for (const frac of [0, 0.5, 1]) {
      const yy = y(top * frac);
      svg.appendChild(el("line", { class: "gridline", x1: pad.l, x2: w - pad.r, y1: yy, y2: yy }));
      const label = Math.round(top * frac) + suffix;
      if (seen.has(label)) continue;
      seen.add(label);
      const t = el("text", { class: "axis", x: 0, y: yy + 4 });
      t.textContent = label;
      svg.appendChild(t);
    }
    if (opts.unit) {
      const t = el("text", { class: "axis axis-unit", x: 0, y: 12 });
      t.textContent = opts.unit;
      svg.appendChild(t);
    }
    return { svg, x, y, w, h, pad, at, head, span, slot: plotW / Math.max(opts.n || 1, 1) };
  }

  // timeAxis labels clock times under the plot, on round steps that fit the
  // width: every 5 to 30 minutes for the hour, every 2 to 6 hours for the day.
  function timeAxis(p, buckets, bars) {
    const n = buckets.length;
    const perMin = (p.w - p.pad.l - p.pad.r) / (n * p.span);
    const step = STEPS.find((s) => s * perMin >= 64) || 1440;
    const base = p.h - p.pad.b;
    for (let i = 0; i < n; i++) {
      const d = new Date(buckets[i].minute * 60000);
      if ((d.getHours() * 60 + d.getMinutes()) % step) continue;
      const xx = bars ? p.x(i, n) - p.slot / 2 : p.x(i, n);
      if (xx + 20 > p.w) continue;
      p.svg.appendChild(el("line", { class: "tick", x1: xx, x2: xx, y1: base, y2: base + 4 }));
      const t = el("text", { class: "axis", x: xx, y: base + 18, "text-anchor": "middle" });
      t.textContent = hm(buckets[i].minute);
      p.svg.appendChild(t);
    }
  }

  // area draws one stacked band. The 2px gap between bands is a stroke in the
  // panel color, so touching fills stay legible instead of merging into a blob.
  function area(p, base, vals, color) {
    const n = vals.length;
    let top = "", bottom = "";
    for (let i = 0; i < n; i++) {
      top += `${i ? "L" : "M"}${p.x(i, n)},${p.y(base[i] + vals[i])}`;
    }
    for (let i = n - 1; i >= 0; i--) {
      bottom += `L${p.x(i, n)},${p.y(base[i])}`;
    }
    p.svg.appendChild(el("path", { d: top + bottom + "Z", fill: color, "fill-opacity": ".55" }));
    p.svg.appendChild(el("path", {
      d: top, fill: "none", stroke: color, "stroke-width": 2, "stroke-linejoin": "round",
    }));
  }

  // bars draws one stacked column per slot. The panel-coloured stroke keeps a
  // gap between columns and between the segments of one column.
  function bars(p, cols) {
    const n = cols[0].length;
    const width = Math.max(1, p.slot - 2);
    for (let i = 0; i < n; i++) {
      let base = 0;
      cols.forEach((col, si) => {
        if (!col[i]) return;
        const y0 = p.y(base), y1 = p.y(base + col[i]);
        p.svg.appendChild(el("rect", {
          class: "bar", x: p.x(i, n) - width / 2, y: y1, width, height: y0 - y1, fill: hue(SERIES[si].color),
        }));
        base += col[i];
      });
    }
  }

  // line draws a rate or an average. A null sample is a minute with nothing to
  // average, which breaks the path: drawing it as zero would claim a 0% hit
  // rate on a cache that was simply never asked.
  function line(p, vals, color) {
    const n = vals.length;
    let d = "", pen = false;
    for (let i = 0; i < n; i++) {
      if (vals[i] === null) {
        pen = false;
        continue;
      }
      d += `${pen ? "L" : "M"}${p.x(i, n)},${p.y(vals[i])}`;
      pen = true;
    }
    p.svg.appendChild(el("path", {
      d, fill: "none", stroke: color, "stroke-width": 2,
      "stroke-linejoin": "round", "stroke-linecap": "round",
    }));

    // A lone sample between two gaps is a path with nowhere to go, and an
    // invisible line is indistinguishable from a broken chart. Mark it.
    for (let i = 0; i < n; i++) {
      if (vals[i] === null) continue;
      const alone = (i === 0 || vals[i - 1] === null) && (i === n - 1 || vals[i + 1] === null);
      if (alone) {
        p.svg.appendChild(el("circle", { cx: p.x(i, n), cy: p.y(vals[i]), r: 3, fill: color }));
      }
    }
  }

  // table fills the chart's data view: hour rows for the day, ten-minute rows
  // for the hour. It is the text alternative, so it carries every total.
  function table(fig, rows, timeHeading) {
    const body = fig.querySelector(".chart-table tbody");
    if (!body) return;
    fig.querySelector("[data-table-time]").textContent = timeHeading;
    body.textContent = "";
    for (const r of rows) {
      const tr = document.createElement("tr");
      const cells = [hm(r.minute), ...SERIES.map((s) => r[s.key] || 0)];
      cells.push(cells.slice(1).reduce((a, b) => a + b, 0));
      for (const c of cells) {
        const td = document.createElement("td");
        td.textContent = c;
        tr.appendChild(td);
      }
      body.appendChild(tr);
    }
  }

  function drawQueries(fig, hour, day) {
    const isDay = view === "day";
    let buckets = isDay ? day || [] : hour;
    let span = isDay ? 10 : 1;
    const svg = fig.querySelector("svg");
    const range = isDay ? "last 24 hours" : "last hour";
    fig.querySelector("[data-window-label]").textContent = "— " + range;
    table(fig, merge(buckets, isDay ? 60 : 10), isDay ? "Hour" : "From");
    if (!buckets.length) {
      return empty(fig, "No history for this window yet", "It fills in as KyDNS answers queries.");
    }
    if (isDay) {
      // On a phone 144 bars would be 2px wide; merge slots so each keeps 4px.
      const plotW = svg.getBoundingClientRect().width - QPAD.l - QPAD.r;
      const k = [1, 2, 3, 6].find((g) => (plotW / buckets.length) * g >= 4) || 6;
      if (k > 1) {
        span *= k;
        buckets = merge(buckets, span);
      }
    }
    const unit = span === 1 ? "per minute" : `per ${span} min`;
    svg.setAttribute("aria-label",
      `Queries ${unit} for the ${range}, split into local, forwarded and blocked`);

    const cols = SERIES.map((s) => buckets.map((b) => b[s.key] || 0));
    const totals = buckets.map((_, i) => cols.reduce((sum, c) => sum + c[i], 0));
    const p = plot(fig, Math.max(0, ...totals), "", false,
      { pad: QPAD, bars: isDay, span, unit: "Queries " + unit, n: buckets.length });
    timeAxis(p, buckets, isDay);

    if (isDay) {
      bars(p, cols);
    } else {
      const base = buckets.map(() => 0);
      SERIES.forEach((s, si) => {
        area(p, base.slice(), cols[si], hue(s.color));
        for (let i = 0; i < base.length; i++) base[i] += cols[si][i];
      });
    }

    // The legend totals the window, not the newest bucket: the current slot
    // is only partly elapsed, so it reads 0 for much of the time.
    fig.querySelectorAll(".legend .key").forEach((k) => {
      const si = SERIES.findIndex((s) => s.key === k.dataset.series);
      k.querySelector("b").textContent = cols[si].reduce((a, b) => a + b, 0);
    });
    const sum = totals.reduce((a, b) => a + b, 0);
    const peak = totals.indexOf(Math.max(...totals));
    svg.setAttribute("aria-description", sum
      ? `${sum} queries; busiest ${p.head(buckets[peak])} with ${totals[peak]}.`
      : "No queries in this window.");
    hover(fig, p, buckets, (b) => SERIES.map((s) => ({
      series: s.key, text: s.label + " " + (b[s.key] || 0),
    })));
  }

  function drawCache(fig, history) {
    const vals = history.map((b) => {
      const t = (b.cache_hits || 0) + (b.cache_misses || 0);
      return t ? Math.round((b.cache_hits * 100) / t) : null;
    });
    if (vals.every((v) => v === null)) {
      return empty(fig, "No cache lookups in this window", "Only forwarded queries use the cache.");
    }
    const p = plot(fig, 100, "%", true);
    line(p, vals, hue("--series-1"));
    hover(fig, p, history, (b, i) =>
      [{ text: vals[i] === null ? "no lookups" : vals[i] + "% hit rate" }]);
  }

  function drawLatency(fig, history) {
    const vals = history.map((b) =>
      (b.latency_count ? Math.round(b.latency_sum_ms / b.latency_count) : null));
    if (vals.every((v) => v === null)) {
      return empty(fig, "No query timings in this window", "Timings appear once queries arrive.");
    }
    const p = plot(fig, Math.max(0, ...vals.filter((v) => v !== null)), "ms");
    line(p, vals, hue("--series-1"));
    hover(fig, p, history, (b, i) =>
      [{ text: vals[i] === null ? "no queries" : vals[i] + " ms average" }]);
  }

  // fillTip writes the tooltip as DOM nodes. rows carry text, never markup, so
  // nothing the endpoint returns is ever parsed as HTML.
  function fillTip(tip, when, rows) {
    tip.textContent = "";
    const head = document.createElement("b");
    head.textContent = when;
    tip.appendChild(head);
    for (const r of rows) {
      tip.appendChild(document.createElement("br"));
      const span = document.createElement("span");
      if (r.series) {
        span.className = "k";
        span.dataset.series = r.series;
      }
      span.textContent = r.text;
      tip.appendChild(span);
    }
  }

  // hover adds the crosshair and tooltip. The hit target is the whole plot
  // area, not the 2px line, so a minute is readable without pixel hunting.
  function hover(fig, p, history, rowsFor) {
    const tip = fig.querySelector(".tip") || fig.appendChild(document.createElement("div"));
    tip.className = "tip";
    tip.style.display = "none";
    const cross = el("line", { class: "crosshair", y1: p.pad.t, y2: p.h - p.pad.b });
    cross.style.display = "none";
    p.svg.appendChild(cross);

    const hit = el("rect", {
      x: p.pad.l, y: p.pad.t, width: p.w - p.pad.l - p.pad.r, height: p.h - p.pad.t - p.pad.b, fill: "transparent",
    });
    p.svg.appendChild(hit);

    hit.addEventListener("mousemove", (e) => {
      const box = p.svg.getBoundingClientRect();
      const vx = ((e.clientX - box.left) / box.width) * p.w; // css px -> viewBox units
      const frac = (vx - p.pad.l) / (p.w - p.pad.l - p.pad.r);
      const i = p.at(Math.min(1, Math.max(0, frac)), history.length);
      const b = history[i];
      cross.setAttribute("x1", p.x(i, history.length));
      cross.setAttribute("x2", p.x(i, history.length));
      cross.style.display = "";
      fillTip(tip, p.head(b), rowsFor(b, i));
      tip.style.display = "";
      tip.style.left = Math.min(box.width - tip.offsetWidth, e.clientX - box.left + 10) + "px";
    });
    hit.addEventListener("mouseleave", () => {
      tip.style.display = "none";
      cross.style.display = "none";
    });
  }

  const DRAW = { queries: drawQueries, cache: drawCache, latency: drawLatency };

  function draw() {
    const [history, day] = latest;
    document.querySelectorAll("[data-chart]").forEach((fig) => {
      const fn = DRAW[fig.dataset.chart];
      if (fn) fn(fig, history, day);
    });
  }

  // The range buttons redraw from the last poll, so switching needs no fetch.
  document.addEventListener("click", (e) => {
    const btn = e.target.closest(".chart-window button");
    if (!btn) return;
    view = btn.dataset.window;
    for (const b of btn.parentElement.children) b.setAttribute("aria-pressed", String(b === btn));
    if (latest) draw();
  });

  window.kydnsDrawCharts = function (history, day) {
    if (!history || !history.length) return;
    latest = [history, day];
    draw();
  };
})();
