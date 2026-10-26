// Staged UI captures against a disposable KyDNS with synthetic data only.
// From the repository root, with a directory holding @playwright/test 1.63 and
// its Chromium (for example a product's web/ after `npm ci`) in CAPTURE_PLAYWRIGHT:
//   CAPTURE_PLAYWRIGHT=/path/to/web node docs/ui-fleet-2026-10-26/capture.mjs <stage nn> <before|after>
// DNS listens on loopback only; queries ask for synthetic local records, never upstream.
import { createHash } from 'node:crypto';
import { createServer } from 'node:http';
import { createRequire } from 'node:module';
import { mkdtemp, mkdir, readFile, rm, writeFile } from 'node:fs/promises';
import { networkInterfaces, release, tmpdir, type } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync, spawn } from 'node:child_process';

const [stage, phase] = process.argv.slice(2);
if (!/^\d\d$/.test(stage ?? '') || !['before', 'after'].includes(phase)) throw new Error('usage: capture.mjs <nn> <before|after>');
const root = fileURLToPath(new URL('../../', import.meta.url));
const here = fileURLToPath(new URL('.', import.meta.url));
const pw = createRequire(join(process.env.CAPTURE_PLAYWRIGHT ?? root, 'package.json'));
const { chromium } = pw('@playwright/test');
const port = process.env.CAPTURE_PORT ?? '5493';
const base = `http://127.0.0.1:${port}`;
const password = 'CaptureAdmin456!';
const dnsPort = process.env.CAPTURE_DNS_PORT ?? '5393';

const dir = await mkdtemp(join(tmpdir(), 'kydns-capture-'));
const binary = join(dir, 'kydns');
execFileSync('go', ['build', '-o', binary, './cmd/kydns'], { cwd: root, stdio: 'inherit' });
await writeFile(join(dir, 'kydns.yaml'), `data_dir: ${join(dir, 'data')}
dns:
  listen: "127.0.0.1:${dnsPort}"
  private_domain: home.arpa
  upstreams: ["tls://1.1.1.1:853"]
  allow_query: [127.0.0.0/8]
admin:
  listen: "127.0.0.1:${port}"
`);
const server = spawn(binary, ['serve', '--config', join(dir, 'kydns.yaml')], { cwd: dir, stdio: ['ignore', 'ignore', 'ignore'], env: { PATH: process.env.PATH, HOME: dir } });
const cleanup = async () => { server.kill('SIGTERM'); await new Promise(r => server.once('exit', r)); await rm(dir, { recursive: true, force: true }); };

function sha(bytes) { return createHash('sha256').update(bytes).digest('hex'); }
function git(...args) { return execFileSync('git', ['-C', root, ...args]); }
// Hash of uncommitted source, excluding this evidence directory.
function worktreeHash() {
  const evidence = ':(exclude)docs/ui-fleet-2026-10-26';
  const tracked = git('diff', 'HEAD', '--binary', '--', '.', evidence);
  const untracked = git('ls-files', '--others', '--exclude-standard', '-z', '--', '.', evidence).toString().split('\0').filter(Boolean);
  if (!tracked.length && !untracked.length) return null;
  const h = createHash('sha256').update(tracked);
  for (const file of untracked.sort()) h.update(file).update(execFileSync('cat', [join(root, file)]));
  return h.digest('hex');
}

// Product theme adapter facts used by the behaviour checks.
const appearancePath = '/appearance';
const storageKey = 'kydns-theme';
const navItem = '.app-nav a.ky-nav-item';
const storedFor = id => ({ coffee: 'Coffee' })[id];
const systemStored = null;
const savedChecks = [['Patina Ky', 'patina-ky', '#e2e8f0'], ['Ocean', 'ocean', '#e0f2fb'], ['Busnes Dark', 'busnes-dark', '']];

const surfaces = {
  dashboard: { path: '/', ready: 'Dashboard' },
  appearance: { path: phase === 'before' ? '/settings' : '/appearance', ready: phase === 'before' ? 'Settings' : 'Appearance' },
};

try {
  const browser = await chromium.launch();
  const setup = await browser.newContext({ baseURL: base });
  const page0 = await setup.newPage();
  for (let i = 0; ; i++) {
    try { await page0.goto('/setup'); break; } catch (e) { if (i > 100) throw e; await new Promise(r => setTimeout(r, 100)); }
  }
  await page0.fill('#token', (await readFile(join(dir, 'data', 'setup-token'), 'utf8')).trim());
  await page0.fill('#password', password);
  await page0.fill('#confirm', password);
  await page0.locator('form button[type=submit]').click();
  await page0.waitForURL(u => !u.pathname.startsWith('/setup'));
  for (const [name, value] of [['nas.home.arpa.', '10.0.0.20'], ['printer.home.arpa.', '10.0.0.31']]) {
    await page0.goto('/records');
    await page0.fill('#rname', name);
    await page0.selectOption('#rtype', 'A');
    await page0.fill('#rvalue', value);
    await page0.locator('form[action="/records/new"] button[type=submit]').click();
    await page0.waitForLoadState();
  }
  for (let i = 0; i < 12; i++) execFileSync('dig', ['@127.0.0.1', '-p', dnsPort, '+short', '+tries=1', '+time=2', i % 3 ? 'nas.home.arpa' : 'printer.home.arpa']);
  const storage = await setup.storageState();
  await setup.close();

  const out = join(here, 'screenshots');
  await mkdir(out, { recursive: true });
  const shots = [];
  for (const theme of ['light', 'dark']) {
    for (const [device, viewport] of [['desktop', { width: 1280, height: 900 }], ['mobile', { width: 390, height: 844 }]]) {
      const ctx = await browser.newContext({ baseURL: base, colorScheme: theme, viewport, deviceScaleFactor: 1, storageState: storage, serviceWorkers: 'block' });
      const page = await ctx.newPage();
      for (const [surface, s] of Object.entries(surfaces)) {
        await page.goto(s.path);
        await page.getByRole('heading', { name: s.ready, exact: true }).first().waitFor();
        await page.evaluate(() => document.fonts.ready);
        await page.waitForTimeout(300);
        const file = `kydns-${stage}-${phase}-${surface}-${device}-${theme}.png`;
        const bytes = await page.screenshot({ path: join(out, file), fullPage: true });
        shots.push({ file, sha256: sha(bytes), surface, url: s.path, device, viewport, theme_source: `OS ${theme} (prefers-color-scheme), no saved choice` });
      }
      await ctx.close();
    }
  }
  // Behaviour checks in the real app (after phase only): each records pass/fail, none aborts.
  const checks = [];
  if (phase === 'after') {
    const ctx = await browser.newContext({ baseURL: base, colorScheme: 'light', viewport: { width: 1280, height: 900 }, storageState: storage });
    const page = await ctx.newPage();
    const errors = [];
    page.on('console', m => { if (m.type() === 'error') errors.push(m.text()); });
    page.on('pageerror', e => errors.push(e.message));
    const check = async (name, fn) => {
      try { checks.push({ name, pass: true, detail: (await fn()) ?? '' }); } catch (e) { checks.push({ name, pass: false, detail: String(e.message ?? e).split('\n')[0] }); }
    };
    const lum = c => { const k = c.startsWith('color(') ? 1 : 255; const [r, g, b] = c.match(/[\d.]+/g).slice(0, 3).map(v => Number(v) / k).map(v => v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4); return 0.2126 * r + 0.7152 * g + 0.0722 * b; };
    const expectEq = (got, want, what) => { if (got !== want) throw new Error(`${what}: got ${got}, want ${want}`); return `${what} = ${got}`; };
    const prop = name => page.evaluate(n => document.documentElement.style.getPropertyValue(n), name);
    const radios = () => page.locator('[data-theme-picker] input[type=radio]');
    await page.goto(appearancePath);
    await radios().first().waitFor();
    await check('all catalog themes plus Match system render as native radios', async () => expectEq(await radios().count(), await page.evaluate(async () => (await import('/static/ky-ui/themes.js')).THEME_CATALOG.length + 1), 'radio count'));
    await check('no saved choice checks Match system and follows the OS', async () => {
      expectEq(await page.locator('input[value=system]').isChecked(), true, 'system checked');
      return expectEq(await page.evaluate(() => document.documentElement.dataset.theme), 'busnes-light', 'data-theme');
    });
    for (const [saved, id, inkStrong] of savedChecks) {
      await check(`legacy saved value ${JSON.stringify(saved)} resolves to ${id} unchanged`, async () => {
        await page.evaluate(([k, v]) => localStorage.setItem(k, v), [storageKey, saved]);
        await page.reload();
        await radios().first().waitFor();
        expectEq(await page.locator(`input[value="${id}"]`).isChecked(), true, `${id} checked`);
        if (inkStrong) expectEq(await prop('--ink-strong'), inkStrong, '--ink-strong');
        return expectEq(await page.evaluate(k => localStorage.getItem(k), storageKey), saved, 'stored value untouched');
      });
    }
    await check('keyboard arrows move the choice and persist it', async () => {
      await page.locator('input[value=ocean]').click(); // focuses even when already checked
      await page.keyboard.press('ArrowRight');
      expectEq(await page.locator('input[value=coffee]').isChecked(), true, 'coffee checked');
      expectEq(await page.locator('input[value=coffee]').evaluate(el => el === document.activeElement), true, 'focus on coffee');
      const outline = await page.locator('label.ky-appearance-option', { has: page.locator('input[value=coffee]') }).evaluate(el => getComputedStyle(el).outlineStyle);
      expectEq(outline, 'solid', 'focus outline');
      return expectEq(await page.evaluate(k => localStorage.getItem(k), storageKey), storedFor('coffee'), 'saved');
    });
    await check('a light named theme keeps light nav hover and selected surfaces', async () => {
      await page.locator('input[value=sky]').check();
      const item = page.locator(navItem).first();
      await item.hover();
      await page.waitForTimeout(300); // nav colours transition
      const [bg, ink] = await item.evaluate(el => [getComputedStyle(el).backgroundColor, getComputedStyle(el).color]);
      if (!(lum(bg) > 0.6 && lum(ink) < 0.2)) throw new Error(`hover ${bg} on ${ink}`);
      const sel = await page.locator(`${navItem}[aria-current="page"]`).evaluate(el => getComputedStyle(el).backgroundColor);
      if (!(lum(sel) > 0.5)) throw new Error(`selected ${sel}`);
      return `hover ${bg} / ${ink}; selected ${sel}`;
    });
    await check('Match system clears the named palette and the saved choice', async () => {
      await page.locator('input[value=system]').check();
      expectEq(await prop('--bg'), '', 'inline --bg');
      expectEq(await page.evaluate(k => localStorage.getItem(k), storageKey), systemStored, 'saved');
      return expectEq(await page.evaluate(() => getComputedStyle(document.body).backgroundColor), 'rgb(248, 246, 240)', 'body background');
    });
    await check('the picker is in sync with another tab', async () => {
      const other = await ctx.newPage();
      await other.goto(appearancePath);
      await other.locator('[data-theme-picker] input[value=forest]').check();
      await page.waitForFunction(() => document.querySelector('[data-theme-picker] input[value=forest]')?.checked);
      await other.close();
      return 'forest checked after storage event';
    });
    await check('no console errors or CSP violations', async () => { if (errors.length) throw new Error(errors.join(' | ')); return 'none'; });
    await ctx.close();
  }

  const manifest = {
    product: 'kydns-server', stage, phase, captured_at: new Date().toISOString(),
    source_commit: git('rev-parse', 'HEAD').toString().trim(), uncommitted_source_sha256: worktreeHash(),
    os: `${type()} ${release()}`, driver: `Playwright ${pw('@playwright/test/package.json').version} Chromium ${browser.version()}, device scale 1, service workers blocked`,
    server: 'kydns built from this worktree into a temporary directory; admin UI and DNS on loopback only; data removed after capture',
    fixture: 'synthetic: admin created through /setup; local A records nas.home.arpa 10.0.0.20 and printer.home.arpa 10.0.0.31; twelve loopback queries for those names; no blocklists, DHCP, replication or upstream traffic',
    shots,
    checks,
  };
  await writeFile(join(here, `manifest-${stage}-${phase}.json`), JSON.stringify(manifest, null, 2) + '\n');
  await browser.close();
  console.log(`${shots.length} captures, ${checks.filter(c => c.pass).length}/${checks.length} checks passed, manifest-${stage}-${phase}.json`);
  for (const c of checks.filter(c => !c.pass)) console.log(`FAIL ${c.name}: ${c.detail}`);
} finally {
  await cleanup();
}
