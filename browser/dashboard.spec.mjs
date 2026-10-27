import { test as base, expect } from '@playwright/test';
import { execFileSync, spawn } from 'node:child_process';
import { once } from 'node:events';
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { createServer } from 'node:net';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../', import.meta.url));
const password = 'BrowserFixtureAdmin456!';

async function unusedPort() {
  const server = createServer();
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const port = server.address().port;
  await new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
  return port;
}

const test = base.extend({
  instance: [async ({ browser }, use) => {
    const dir = await mkdtemp(join(tmpdir(), 'dns-ui-'));
    let child;
    let stopped;
    let diagnostics = '';
    try {
      const binary = join(dir, 'kydns');
      execFileSync('go', ['build', '-o', binary, './cmd/kydns'], { cwd: root, stdio: 'pipe', timeout: 120_000 });
      const admin = await unusedPort();
      const dns = await unusedPort();
      const url = `http://127.0.0.1:${admin}`;
      await writeFile(join(dir, 'config.yaml'), `data_dir: ${join(dir, 'data')}
dns:
  listen: "127.0.0.1:${dns}"
  upstreams: ["udp://127.0.0.1:9"]
  allow_query: [127.0.0.0/8]
admin:
  listen: "127.0.0.1:${admin}"
replication:
  listen: "127.0.0.1:0"
`);
      child = spawn(binary, ['serve', '--config', join(dir, 'config.yaml')], {
        cwd: dir, env: { PATH: process.env.PATH, HOME: dir }, stdio: ['ignore', 'pipe', 'pipe'],
      });
      stopped = once(child, 'exit');
      child.stdout.on('data', chunk => { diagnostics = (diagnostics + chunk).slice(-8192); });
      child.stderr.on('data', chunk => { diagnostics = (diagnostics + chunk).slice(-8192); });
      await expect.poll(async () => {
        if (child.exitCode !== null) throw new Error(diagnostics);
        try { return (await fetch(url + '/healthz', { signal: AbortSignal.timeout(500) })).status; }
        catch { return 0; }
      }, { timeout: 20_000 }).toBe(200);

      // Bootstrap through the real form. Bad credentials must not consume setup.
      const context = await browser.newContext();
      try {
        const page = await context.newPage();
        await page.goto(url + '/setup');
        const token = (await readFile(join(dir, 'data/setup-token'), 'utf8')).trim();
        for (const [given, confirm, error] of [
          ['wrong-token', password, 'That setup token is not correct.'],
          [token, 'different-password', 'The two passwords do not match.'],
        ]) {
          await page.locator('#token').fill(given);
          await page.locator('#password').fill(password);
          await page.locator('#confirm').fill(confirm);
          await page.getByRole('button', { name: 'Create admin account' }).click();
          await expect(page.locator('.error')).toContainText(error);
        }
        await page.locator('#token').fill(token);
        await page.locator('#password').fill(password);
        await page.locator('#confirm').fill(password);
        await page.getByRole('button', { name: 'Create admin account' }).click();
        await expect(page).toHaveURL(url + '/');
        await use({ url, storageState: await context.storageState() });
      } finally { await context.close(); }
    } finally {
      if (child && child.exitCode === null && child.signalCode === null) {
        child.kill('SIGTERM');
        const timeout = setTimeout(() => child.kill('SIGKILL'), 5000);
        try { await stopped; } finally { clearTimeout(timeout); }
      }
      await rm(dir, { recursive: true, force: true });
    }
  }, { scope: 'worker', timeout: 180_000 }],
  storageState: async ({ instance }, use) => use(instance.storageState),
  baseURL: async ({ instance }, use) => use(instance.url),
  browserErrors: [async ({ context, instance }, use) => {
    const errors = [];
    context.on('page', page => page.on('pageerror', error => errors.push(error.message)));
    await context.route('**/*', route => {
      if (new URL(route.request().url()).origin === instance.url) return route.continue();
      errors.push('Unexpected external browser request: ' + route.request().url());
      return route.abort();
    });
    await use(errors);
    expect(errors).toEqual([]);
  }, { auto: true }],
});

function stats(history = []) {
  return {
    queries: { total: 37, authoritative: 20, forwarded: 10, blocked: 7, nxdomain: 2, servfail: 1, avg_ms: 3, last_query: 0 },
    cache: { hit_rate: 75, entries: 12 }, refusals: { total: 4, cgnat: 3 }, uptime_seconds: 90061, history,
  };
}

async function mockStats(page, value) {
  await page.route('**/stats.json', route => route.fulfill({ json: value }));
  await page.goto('/');
  await expect(page.locator('[data-stat="total"]')).toHaveText('37');
}

test('real dashboard counters refresh without query traffic', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Dashboard', exact: true })).toBeVisible();
  const response = await page.request.get('/stats.json');
  expect(response.ok()).toBeTruthy();
  const actual = await response.json();
  expect(actual.history).toHaveLength(60);
  expect(actual.queries.total).toBe(0);
  await expect(page.locator('#chart-queries svg path')).toHaveCount(6);
  await expect(page.locator('[data-stat="last_query"]')).toHaveText('never');
});

test('stacked charts total the full window and separate missing samples', async ({ page }) => {
  await mockStats(page, stats([
    { minute: 100, authoritative: 2, forwarded: 3, blocked: 1, cache_hits: 1, cache_misses: 1, latency_count: 2, latency_sum_ms: 6 },
    { minute: 101 },
    { minute: 102, authoritative: 5, forwarded: 1, blocked: 2, cache_hits: 3, cache_misses: 1, latency_count: 1, latency_sum_ms: 1 },
  ]));
  await expect(page.locator('[data-series="authoritative"] b')).toHaveText('7');
  await expect(page.locator('[data-series="forwarded"] b')).toHaveText('4');
  await expect(page.locator('[data-series="blocked"] b')).toHaveText('3');
  for (const id of ['cache', 'latency']) {
    await expect(page.locator(`#chart-${id} circle`)).toHaveCount(2);
    const d = await page.locator(`#chart-${id} path`).getAttribute('d');
    expect(d.match(/M/g)).toHaveLength(2);
    expect(d).not.toMatch(/NaN|Infinity|L/);
  }
  await expect(page.locator('[data-stat="uptime"]')).toHaveText('1d 1h');
  await expect(page.locator('[data-stat="refused_cgnat"]')).toHaveText('3');
  const svg = page.locator('#chart-queries svg');
  const width = (await svg.boundingBox()).width;
  expect(Number((await svg.getAttribute('viewBox')).split(' ')[2])).toBe(Math.round(width));
  await expect(page.locator('#chart-cache .axis')).toHaveText(['0%', '50%', '100%']);
});

test('hover shows the selected minute and clears crosshair on leave', async ({ page }) => {
  await mockStats(page, stats([{ minute: 100 }, { minute: 101, cache_hits: 1, cache_misses: 3 }]));
  const svg = page.locator('#chart-cache svg');
  await svg.scrollIntoViewIfNeeded();
  const box = await svg.boundingBox();
  await page.mouse.move(box.x + box.width - 10, box.y + 35);
  await expect(page.locator('#chart-cache .tip')).toBeVisible();
  await expect(page.locator('#chart-cache .tip')).toContainText('25% hit rate');
  // A painted vertical SVG line has a zero-width bounding box.
  await expect(page.locator('#chart-cache .crosshair')).toHaveCSS('display', 'inline');
  await page.mouse.move(0, 0);
  await expect(page.locator('#chart-cache .tip')).toBeHidden();
  await expect(page.locator('#chart-cache .crosshair')).toHaveCSS('display', 'none');
});

test('single sample and zero data have finite coordinates and unique axis labels', async ({ page }) => {
  await mockStats(page, stats([{ minute: 100, cache_hits: 1, cache_misses: 0, latency_count: 1, latency_sum_ms: 1 }]));
  await expect(page.locator('#chart-cache circle')).toHaveCount(1);
  await expect(page.locator('#chart-latency .axis')).toHaveText(['0ms', '1ms']);
  const geometry = await page.locator('svg path, svg circle').evaluateAll(nodes => nodes.map(n => n.outerHTML).join(''));
  expect(geometry).not.toMatch(/NaN|Infinity/);
});

for (const failure of ['http', 'json', 'network']) {
  test(`live refresh retains data after ${failure} failure and recovers`, async ({ page }) => {
    await page.clock.install();
    let calls = 0;
    await page.route('**/stats.json', route => {
      calls++;
      if (calls !== 2) {
        const value = stats();
        if (calls > 2) value.queries.total = 38;
        return route.fulfill({ json: value });
      }
      if (failure === 'network') return route.abort();
      return route.fulfill({ status: failure === 'http' ? 503 : 200, contentType: 'application/json', body: '{broken' });
    });
    await page.goto('/');
    await expect(page.locator('[data-stat="total"]')).toHaveText('37');
    await page.clock.runFor(5000);
    await expect.poll(() => calls).toBe(2);
    await expect(page.locator('[data-stat="total"]')).toHaveText('37');
    await page.clock.runFor(5000);
    await expect.poll(() => calls).toBe(3);
    await expect(page.locator('[data-stat="total"]')).toHaveText('38');
  });
}

test('401 stops dashboard polling', async ({ page }) => {
  await page.clock.install();
  // Count invocation in the browser too: routed HTTP requests can still be
  // queued in the test driver when the simulated clock finishes advancing.
  await page.addInitScript(() => {
    const original = window.fetch;
    window.statsFetches = 0;
    window.fetch = function (...args) {
      if (args[0] === '/stats.json') window.statsFetches++;
      return original.apply(this, args);
    };
  });
  let calls = 0;
  await page.route('**/stats.json', route => { calls++; return route.fulfill({ status: 401, body: '' }); });
  await page.goto('/');
  await expect.poll(() => calls).toBe(1);
  await page.clock.runFor(15_000);
  expect(await page.evaluate(() => window.statsFetches)).toBe(1);
  expect(calls).toBe(1);
});

test('Appearance persists names, follows OS and synchronizes another tab', async ({ page, context }) => {
  await page.emulateMedia({ colorScheme: 'light' });
  await page.goto('/appearance');
  const radios = page.locator('[data-theme-picker] input');
  const catalog = await page.evaluate(() => window.kyThemes.THEME_CATALOG);
  await expect(radios).toHaveCount(catalog.length + 1);
  await expect(page.locator('input[value="system"]')).toBeChecked();
  await expect(page.locator('html')).toHaveAttribute('style', /color-scheme: light/);
  await page.emulateMedia({ colorScheme: 'dark' });
  await expect(page.locator('html')).toHaveAttribute('style', /color-scheme: dark/);
  await page.locator('input[value="ocean"]').check();
  expect(await page.evaluate(() => localStorage.getItem('kydns-theme'))).toBe('Ocean');
  await page.reload();
  await expect(page.locator('input[value="ocean"]')).toBeChecked();
  const second = await context.newPage();
  await second.goto('/appearance');
  await second.locator('input[value="coffee"]').check();
  await expect(page.locator('input[value="coffee"]')).toBeChecked();
  await second.locator('input[value="system"]').check();
  await expect(page.locator('input[value="system"]')).toBeChecked();
  expect(await page.evaluate(() => localStorage.getItem('kydns-theme'))).toBeNull();
});

test('Appearance applies choices when browser storage is unavailable', async ({ page }) => {
  await page.addInitScript(() => Object.defineProperty(window, 'localStorage', { get() { throw new DOMException('disabled', 'SecurityError'); } }));
  await page.goto('/appearance');
  await page.locator('input[value="busnes-dark"]').check();
  await expect(page.locator('html')).toHaveAttribute('style', /color-scheme: dark/);
});

test('manual-record errors, addition and removal use actual server forms', async ({ page }) => {
  await page.goto('/records');
  await page.locator('#rname').fill('browser.home.arpa.');
  await page.locator('#rtype').selectOption('A');
  await page.locator('#rvalue').fill('invalid-address');
  await page.getByRole('button', { name: 'Add record', exact: true }).click();
  await expect(page.locator('.error')).toBeVisible();
  await expect(page.getByRole('row').filter({ hasText: 'browser.home.arpa.' })).toHaveCount(0);
  await page.locator('#rname').fill('browser.home.arpa.');
  await page.locator('#rtype').selectOption('A');
  await page.locator('#rvalue').fill('192.0.2.42');
  await page.getByRole('button', { name: 'Add record', exact: true }).click();
  const row = page.getByRole('row').filter({ hasText: 'browser.home.arpa.' });
  await expect(row).toContainText('192.0.2.42');
  await row.getByRole('button', { name: 'Remove' }).click();
  await expect(row).toHaveCount(0);
});

test('signed-out visitors cannot read the dashboard or Appearance', async ({ browser, instance }) => {
  const context = await browser.newContext({ storageState: { cookies: [], origins: [] } });
  try {
    const page = await context.newPage();
    for (const path of ['/', '/appearance']) {
      await page.goto(instance.url + path);
      await expect(page).toHaveURL(instance.url + '/login');
      await expect(page.getByRole('heading', { name: 'Sign in', exact: true })).toBeVisible();
    }
    expect((await context.request.get(instance.url + '/stats.json')).status()).toBe(401);
  } finally { await context.close(); }
});

test('admin layouts retain reachable fields and routing actions at shipped sizes', async ({ page }) => {
  await page.goto('/discovered');
  const phone = page.viewportSize().width < 860;
  const nav = await page.locator('nav').boundingBox();
  if (phone) expect(nav.height).toBeLessThan(160);
  else expect(nav.width).toBe(240);

  await page.goto('/settings');
  for (const group of await page.locator('.field-set').all()) {
    const bounds = await group.boundingBox();
    expect(bounds.x).toBeGreaterThanOrEqual(0);
    expect(bounds.x + bounds.width).toBeLessThanOrEqual(page.viewportSize().width);
  }
  await expect(page.getByRole('button', { name: 'Save settings', exact: true })).toBeInViewport();

  await page.goto('/services');
  await page.locator('#name').fill('layout-check');
  await page.locator('#address').fill('192.0.2.42');
  await page.getByRole('button', { name: 'Add service', exact: true }).click();
  const row = page.getByRole('row').filter({ hasText: 'layout-check' });
  try {
    await expect(row.getByRole('button', { name: 'Remove', exact: true })).toBeVisible();
    const disclosure = row.locator('summary');
    await disclosure.focus();
    await disclosure.press('Enter');
    const proxy = row.getByRole('textbox', { name: 'Proxy address for layout-check', exact: true });
    await expect(proxy).toBeVisible();
    await proxy.fill('192.0.2.43');
    await row.getByRole('checkbox', { name: 'Route', exact: true }).check();
    await row.getByRole('button', { name: 'Save', exact: true }).click();
    await expect(row.locator('summary')).toHaveText('Via proxy');
    await row.locator('summary').click();
    await expect(proxy).toHaveValue('192.0.2.43');
    await expect(row.getByRole('checkbox', { name: 'Route', exact: true })).toBeChecked();
    if (phone) {
      for (const control of await row.locator('input:not([type=hidden]),button,summary').all()) {
        const bounds = await control.boundingBox();
        expect(bounds.x).toBeGreaterThanOrEqual(0);
        expect(bounds.x + bounds.width).toBeLessThanOrEqual(page.viewportSize().width);
      }
    }
  } finally {
    await row.getByRole('button', { name: 'Remove', exact: true }).click();
    await expect(row).toHaveCount(0);
  }
});
