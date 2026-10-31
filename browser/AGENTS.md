# Purpose

Chromium regressions for the shipped KyDNS dashboard, live refresh, Appearance pairs, the About dialog, the Records add dialog, filter and row menu, and responsive administrator layouts.

## Ownership

- Tests own disposable loopback servers, temporary data and synthetic credentials.
- Production templates, scripts and behavior remain owned by the repository root.

## Local Contracts

- Build the current Go source; use real setup, sessions and record handlers.
- Mock only dashboard data/failure responses for deterministic chart and polling cases.
- Keep listeners and fixture upstreams on loopback. Block external browser requests.
- Never accept a live service URL, persistent data directory or real credential.
- Stop child servers and remove owned data after success or failure; retain failure traces.
- These targeted tests do not establish most-branches/input/functionality coverage.

## Work Guidance

- Use Playwright's assertions and clock; no fixed sleeps or retries hiding failures.
- The responsive administrator regression checks short-page navigation and its scroll cue, the four Dashboard tiles, Settings containment and top Save, then opens service routing by keyboard and persists a real edit. Keep the owned service cleanup in its `finally` block.
- The empty-chart regression uses a real stats refresh to distinguish no samples from valid 0% cache hits and 0ms latency, and verifies the visible and accessible empty descriptions clear.
- Keep the package private and the browser tool dependency exactly pinned.

## Verification

- `npm ci && npx playwright install chromium && npm test` in this directory; CI runs it in the pinned `mcr.microsoft.com/playwright` image of the lockfile's version instead of installing browsers.
- CI and tag releases run the suite before publishing.

## Child DOX Index
