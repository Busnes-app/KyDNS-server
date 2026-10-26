// Visual refinement captures against a disposable KyDNS with synthetic data only.
// From the repository root, with a directory holding @playwright/test and its Chromium
// (for example a product's web/ after `npm ci`) in CAPTURE_PLAYWRIGHT:
//   CAPTURE_PLAYWRIGHT=/path/to/web node docs/ui-visual-refinement-2026-10-26/capture.mjs <stage nn> <before|after>
// DNS listens on loopback only; queries ask for synthetic local records, never upstream.
import { createHash } from 'node:crypto';
import { createRequire } from 'node:module';
import { connect } from 'node:net';
import { mkdtemp, mkdir, readFile, rm, writeFile } from 'node:fs/promises';
import { release, tmpdir, type } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync, spawn } from 'node:child_process';

const [stage, phase] = process.argv.slice(2);
if (!/^\d\d$/.test(stage ?? '') || !['before', 'after'].includes(phase)) throw new Error('usage: capture.mjs <nn> <before|after>');
const evidenceDir = 'docs/ui-visual-refinement-2026-10-26';
const root = fileURLToPath(new URL('../../', import.meta.url));
const here = fileURLToPath(new URL('.', import.meta.url));
const pw = createRequire(join(process.env.CAPTURE_PLAYWRIGHT ?? root, 'package.json'));
const { chromium } = pw('@playwright/test');
const port = process.env.CAPTURE_PORT ?? '5391';
const base = `http://127.0.0.1:${port}`;
const password = 'CaptureAdmin456!';
const dnsPort = process.env.CAPTURE_DNS_PORT ?? '5393';

// Refuse to start when another process holds a port: the run would capture someone else's server.
for (const p of [port, dnsPort]) {
  const busy = await new Promise(r => { const s = connect(+p, '127.0.0.1', () => { s.destroy(); r(true); }); s.on('error', () => r(false)); });
  if (busy) throw new Error(`127.0.0.1:${p} is in use by another process; set CAPTURE_PORT`);
}
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
const exited = new Promise(r => server.once('exit', r));
const cleanup = async () => { server.kill('SIGTERM'); await exited; await rm(dir, { recursive: true, force: true }); };

function sha(bytes) { return createHash('sha256').update(bytes).digest('hex'); }
function git(...args) { return execFileSync('git', ['-C', root, ...args]); }
// Hash of uncommitted source, excluding this evidence directory.
function worktreeHash() {
  const evidence = `:(exclude)${evidenceDir}`;
  const tracked = git('diff', 'HEAD', '--binary', '--', '.', evidence);
  const untracked = git('ls-files', '--others', '--exclude-standard', '-z', '--', '.', evidence).toString().split('\0').filter(Boolean);
  if (!tracked.length && !untracked.length) return null;
  const h = createHash('sha256').update(tracked);
  for (const file of untracked.sort()) h.update(file).update(execFileSync('cat', [join(root, file)]));
  return h.digest('hex');
}

// Appearance is the last navigation row, so its phone capture shows whether the current row is in view.
const surfaces = {
  dashboard: { path: '/', ready: 'Dashboard' },
  appearance: { path: '/appearance', ready: 'Appearance' },
};
const open = async (page, s) => {
  await page.goto(s.path);
  await page.getByRole('heading', { name: s.ready, exact: true }).first().waitFor();
  // The dashboard draws its charts after the first /stats.json poll.
  if (s.path === '/') await page.locator('#chart-queries svg text').first().waitFor();
  await page.evaluate(() => document.fonts.ready);
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
  let fonts = '';
  for (const theme of ['light', 'dark']) {
    for (const [device, viewport] of [['desktop', { width: 1280, height: 900 }], ['mobile', { width: 390, height: 844 }]]) {
      const ctx = await browser.newContext({ baseURL: base, colorScheme: theme, viewport, deviceScaleFactor: 1, storageState: storage, serviceWorkers: 'block' });
      const page = await ctx.newPage();
      for (const [surface, s] of Object.entries(surfaces)) {
        await open(page, s);
        fonts ||= await page.evaluate(() => `Space Grotesk loaded: ${document.fonts.check('15px "Space Grotesk"')}; IBM Plex Mono loaded: ${document.fonts.check('13px "IBM Plex Mono"')}`);
        await page.waitForTimeout(300);
        const file = `kydns-${stage}-${phase}-${surface}-${device}-${theme}.png`;
        const bytes = await page.screenshot({ path: join(out, file), fullPage: true });
        shots.push({ file, sha256: sha(bytes), surface, url: s.path, device, viewport, theme_source: `OS ${theme} (prefers-color-scheme), no saved choice` });
      }
      await ctx.close();
    }
  }

  // Behaviour checks in the real app, both phases: each records pass/fail, none aborts.
  const checks = [];
  const check = async (name, fn) => {
    try { checks.push({ name, pass: true, detail: (await fn()) ?? '' }); } catch (e) { checks.push({ name, pass: false, detail: String(e.message ?? e).split('\n')[0] }); }
  };
  const errors = [];
  const phone = await browser.newContext({ baseURL: base, colorScheme: 'light', viewport: { width: 390, height: 844 }, storageState: storage });
  phone.on('page', p => { p.on('console', m => { if (m.type() === 'error') errors.push(m.text()); }); p.on('pageerror', e => errors.push(e.message)); });
  const p = await phone.newPage();
  await check('phone: header height (top of page to bottom of navigation)', async () => {
    await open(p, surfaces.dashboard);
    return `${await p.evaluate(() => Math.round(document.querySelector('.app-nav').getBoundingClientRect().bottom))}px`;
  });
  await check('phone: chart axis labels render at 11px or more', async () => {
    const sizes = await p.evaluate(() => [...document.querySelectorAll('.chart svg text.axis')].map(t => +t.getBoundingClientRect().height.toFixed(1)));
    if (!sizes.length || Math.min(...sizes) < 11) throw new Error(`label box heights ${JSON.stringify(sizes)}`);
    return `label box heights ${Math.min(...sizes)}-${Math.max(...sizes)}px`;
  });
  await check('phone: stat values share column edges', async () => {
    const rows = await p.evaluate(() => [...document.querySelectorAll('.stat-row')].map(r => [...new Set([...r.querySelectorAll('.stat .value')].map(v => Math.round(v.getBoundingClientRect().left)))]));
    const bad = rows.filter(r => r.length > 2);
    if (bad.length) throw new Error(`value left edges per row ${JSON.stringify(rows)}`);
    return `value left edges per row ${JSON.stringify(rows)}`;
  });
  await check('phone: the current navigation row is in view on every page', async () => {
    const seen = [];
    for (const s of Object.values(surfaces)) {
      await open(p, s);
      const r = await p.evaluate(() => {
        const cur = document.querySelector('.app-nav [aria-current="page"]'), i = cur.getBoundingClientRect();
        return { label: cur.textContent.trim(), inView: i.left >= 0 && i.right <= innerWidth, y: scrollY, focus: document.activeElement === document.body };
      });
      if (!r.inView || r.y !== 0 || !r.focus) throw new Error(JSON.stringify(r));
      seen.push(r.label);
    }
    return `${seen.join(', ')} in view; page not scrolled; focus on body`;
  });
  await check('phone: the version and licence button opens About', async () => {
    await open(p, surfaces.dashboard);
    const b = p.getByRole('button', { name: /MIT License/ }).filter({ visible: true });
    await b.click();
    await p.locator('#about:popover-open').waitFor();
    await p.keyboard.press('Escape');
    return `${await b.count()} visible trigger; About opened`;
  });
  await check('no console errors or CSP violations', async () => { if (errors.length) throw new Error(errors.join(' | ')); return 'none'; });
  await phone.close();

  const manifest = {
    product: 'kydns-server', stage, phase, captured_at: new Date().toISOString(),
    source_commit: git('rev-parse', 'HEAD').toString().trim(), uncommitted_source_sha256: worktreeHash(),
    os: `${type()} ${release()}`, driver: `Playwright ${pw('@playwright/test/package.json').version} Chromium ${browser.version()}, device scale 1, service workers blocked`,
    fonts: `self-hosted Space Grotesk and IBM Plex Mono from /static/fonts; ${fonts}`,
    server: `kydns built from this worktree into a temporary directory; admin UI on 127.0.0.1:${port} and DNS on 127.0.0.1:${dnsPort}; data removed after capture`,
    fixture: 'synthetic: admin created through /setup; local A records nas.home.arpa 10.0.0.20 and printer.home.arpa 10.0.0.31; twelve loopback queries for those names; no blocklists, DHCP, replication or upstream traffic. Uptime and last-query ages are live and differ between runs',
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
