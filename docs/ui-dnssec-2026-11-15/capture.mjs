// Before/after captures for "Validate DNSSEC locally" against a disposable
// KyDNS on loopback, synthetic data only, no DNS query leaves the machine.
//   CAPTURE_PLAYWRIGHT=browser node docs/ui-dnssec-2026-11-15/capture.mjs <source dir> <before|after>
// "after" arms the one-time upgrade notice, as the migration leaves it.
import { createRequire } from 'node:module';
import { mkdtemp, mkdir, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync, spawn } from 'node:child_process';
import { DatabaseSync } from 'node:sqlite';

const [src, phase] = process.argv.slice(2);
if (!src || !['before', 'after'].includes(phase)) throw new Error('usage: capture.mjs <source dir> <before|after>');
const here = fileURLToPath(new URL('.', import.meta.url));
const pw = createRequire(join(resolve(process.env.CAPTURE_PLAYWRIGHT ?? 'browser'), 'package.json'));
const { chromium } = pw('@playwright/test');
const port = '5391', dnsPort = '5392', base = `http://127.0.0.1:${port}`;
const password = 'CaptureAdmin456!';

const dir = await mkdtemp(join(tmpdir(), 'kydns-dnssec-capture-'));
execFileSync('go', ['build', '-o', join(dir, 'kydns'), './cmd/kydns'], { cwd: resolve(src), stdio: 'inherit' });
await writeFile(join(dir, 'kydns.yaml'), `data_dir: ${join(dir, 'data')}
dns:
  listen: "127.0.0.1:${dnsPort}"
  upstreams: ["tls://192.0.2.1:853"]
  allow_query: [127.0.0.0/8]
admin:
  listen: "127.0.0.1:${port}"
`);
const env = { PATH: process.env.PATH, HOME: dir, PUSH_RELAY_URL: '', APNS_RELAY_URL: '' };
const server = spawn(join(dir, 'kydns'), ['serve', '--config', join(dir, 'kydns.yaml')], { cwd: dir, stdio: 'ignore', env });
try {
  const browser = await chromium.launch();
  const setup = await browser.newContext({ baseURL: base });
  const p = await setup.newPage();
  await p.route(u => !u.href.startsWith(base), r => r.abort());
  for (let i = 0; ; i++) {
    try { await p.goto('/setup'); break; } catch (e) { if (i > 100) throw e; await new Promise(r => setTimeout(r, 100)); }
  }
  await p.fill('#token', (await readFile(join(dir, 'data', 'setup-token'), 'utf8')).trim());
  await p.fill('#password', password);
  await p.fill('#confirm', password);
  await p.locator('form button[type=submit]').click();
  await p.waitForURL(u => !u.pathname.startsWith('/setup'));
  if (phase === 'after') {
    const db = new DatabaseSync(join(dir, 'data', 'kydns.db'));
    db.exec('UPDATE settings SET dnssec_notice = 1, dnssec_validate = 0');
    db.close();
    // The running snapshot still says on; a save reloads it from the row.
    await p.goto('/settings/server');
    await p.locator('input[name=dnssec_validate]').uncheck();
    await p.getByRole('button', { name: 'Save settings' }).click();
    await p.waitForLoadState();
  }
  const storage = await setup.storageState();
  await setup.close();
  const out = join(here, 'screenshots');
  await mkdir(out, { recursive: true });
  for (const theme of ['light', 'dark']) {
    for (const [device, viewport] of [['desktop', { width: 1280, height: 900 }], ['phone', { width: 390, height: 844 }]]) {
      const ctx = await browser.newContext({ baseURL: base, colorScheme: theme, viewport, deviceScaleFactor: 1, storageState: storage, serviceWorkers: 'block' });
      const page = await ctx.newPage();
      await page.route(u => !u.href.startsWith(base), r => r.abort());
      for (const [surface, path] of [['dashboard', '/'], ['settings-server', '/settings/server']]) {
        await page.goto(path);
        await page.evaluate(() => document.fonts.ready);
        await page.waitForTimeout(300);
        await page.screenshot({ path: join(out, `${phase}-${surface}-${device}-${theme}.png`), fullPage: true });
      }
      await ctx.close();
    }
  }
  await browser.close();
} finally {
  server.kill('SIGTERM');
  await new Promise(r => server.once('exit', r));
  await rm(dir, { recursive: true, force: true });
}
