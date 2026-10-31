// Refreshes dashboard stats and the lease table.
(function () {
  const LEASE_MS = 30000;
  const STATS_MS = 5000;

  let statsTimer = null;

  // The dashboard's numbers are already correct when the page renders; this
  // keeps them moving so a working server does not look like a stopped one.
  function setStat(name, value) {
    for (const node of document.querySelectorAll('[data-stat="' + name + '"]')) {
      node.textContent = value;
    }
  }

  // Mirrors shortDuration in humanize.go: one useful unit, never "76h12m9.4s".
  function shortDuration(secs) {
    if (secs < 60) return Math.floor(secs) + "s";
    if (secs < 3600) return Math.floor(secs / 60) + "m";
    if (secs < 86400) return Math.floor(secs / 3600) + "h";
    return Math.floor(secs / 86400) + "d " + (Math.floor(secs / 3600) % 24) + "h";
  }

  async function refreshStats() {
    try {
      const resp = await fetch("/stats.json", { headers: { Accept: "application/json" } });
      if (resp.status === 401) {
        clearInterval(statsTimer);
        return;
      }
      if (!resp.ok) return;
      const d = await resp.json();
      for (const k of ["total", "authoritative", "forwarded", "blocked", "nxdomain", "servfail"]) {
        setStat(k, d.queries[k]);
      }
      setStat("avg_ms", d.queries.avg_ms);
      // Mirrors MetricsSnapshot.BlockedRate: rounded down, 0 before the first query.
      setStat("blocked_rate", d.queries.total ? Math.floor((d.queries.blocked * 100) / d.queries.total) : 0);
      setStat("hit_rate", d.cache.hit_rate);
      setStat("cache_entries", d.cache.entries);
      setStat("uptime", shortDuration(d.uptime_seconds));
      setStat("last_query", d.queries.last_query
        ? shortDuration(Date.now() / 1000 - d.queries.last_query) + " ago"
        : "never");
      if (d.refusals) {
        setStat("refused_total", d.refusals.total);
        setStat("refused_cgnat", d.refusals.cgnat);
      }
      if (window.kydnsDrawCharts) window.kydnsDrawCharts(d.history);
    } catch (e) {
      /* transient: the next tick tries again */
    }
  }

  if (document.querySelector("[data-chart]")) {
    refreshStats();
    statsTimer = setInterval(refreshStats, STATS_MS);
  }

  if (document.getElementById("lease-table")) {
    setInterval(function () { location.reload(); }, LEASE_MS);
  }
})();
