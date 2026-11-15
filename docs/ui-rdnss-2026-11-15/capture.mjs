// DHCP tab captures for the IPv6 DNS advertisement card, against a disposable
// KyDNS with synthetic data. From the repository root, after `npm ci` in browser/:
//   node docs/ui-rdnss-2026-11-15/capture.mjs <before|after> <path to a built kydns>
// Loopback only; no query leaves the machine.
import { createRequire } from 'node:module';
import { mkdtemp, mkdir, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawn } from 'node:child_process';

const [phase, binary] = process.argv.slice(2);
if (!['before', 'after'].includes(phase) || !binary) throw new Error('usage: capture.mjs <before|after> <kydns>');
const root = fileURLToPath(new URL('../../', import.meta.url));
const here = fileURLToPath(new URL('.', import.meta.url));
const { chromium } = createRequire(join(root, 'browser', 'package.json'))('@playwright/test');
const port = process.env.CAPTURE_PORT ?? '5391';
const dnsPort = process.env.CAPTURE_DNS_PORT ?? '5392';
const base = `http://127.0.0.1:${port}`;
const password = 'CaptureAdmin456!';

const dir = await mkdtemp(join(tmpdir(), 'kydns-rdnss-capture-'));
await writeFile(join(dir, 'kydns.yaml'), `data_dir: ${join(dir, 'data')}
dns:
  listen: "127.0.0.1:${dnsPort}"
  private_domain: home.arpa
  upstreams: ["tls://127.0.0.1:1"]
  allow_query: [127.0.0.0/8]
admin:
  listen: "127.0.0.1:${port}"
`);
const server = spawn(binary, ['serve', '--config', join(dir, 'kydns.yaml')], {
  cwd: dir, stdio: 'ignore', env: { PATH: process.env.PATH, HOME: dir, PUSH_RELAY_URL: '', APNS_RELAY_URL: '' },
});
try {
  const browser = await chromium.launch();
  const setup = await browser.newContext({ baseURL: base });
  const p = await setup.newPage();
  for (let i = 0; ; i++) {
    try { await p.goto('/setup'); break; } catch (e) { if (i > 100) throw e; await new Promise(r => setTimeout(r, 100)); }
  }
  await p.fill('#token', (await readFile(join(dir, 'data', 'setup-token'), 'utf8')).trim());
  await p.fill('#password', password);
  await p.fill('#confirm', password);
  await p.locator('form button[type=submit]').click();
  await p.waitForURL(u => !u.pathname.startsWith('/setup'));
  const storage = await setup.storageState();
  await setup.close();
  const out = join(here, 'screenshots');
  await mkdir(out, { recursive: true });
  for (const theme of ['light', 'dark']) {
    for (const [device, viewport] of [['desktop', { width: 1280, height: 900 }], ['mobile', { width: 390, height: 844 }]]) {
      const ctx = await browser.newContext({ baseURL: base, colorScheme: theme, viewport, deviceScaleFactor: 1, storageState: storage });
      const page = await ctx.newPage();
      await page.goto('/dhcp');
      await page.getByRole('heading', { name: 'DHCP', exact: true }).first().waitFor();
      await page.evaluate(() => document.fonts.ready);
      const file = `dhcp-${phase}-${device}-${theme}.png`;
      await page.screenshot({ path: join(out, file), fullPage: true });
      console.log(file);
      await ctx.close();
    }
  }
  await browser.close();
} finally {
  server.kill('SIGTERM');
  await new Promise(r => server.once('exit', r));
  await rm(dir, { recursive: true, force: true });
}
