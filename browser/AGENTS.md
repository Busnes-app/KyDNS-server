# Purpose

Chromium regressions for the shipped KyDNS dashboard, live refresh, Appearance and record forms.

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
- Keep the package private and the browser tool dependency exactly pinned.

## Verification

- `npm ci && npx playwright install chromium && npm test` in this directory.
- CI and tag releases run the suite before publishing.

## Child DOX Index
