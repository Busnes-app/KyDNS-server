// Exact-DPR round4 matched after capture, against the already-owned real fixture. See baseline-setup.md.
// No server build/start, fixture mutation, auth injection, network interception or retries.
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { access, chmod, mkdir, readFile, rename, stat, writeFile } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { release, type } from 'node:os';

const here = dirname(fileURLToPath(import.meta.url));
const root = fileURLToPath(new URL('../../', import.meta.url));
const frozen = '2eb750d46ad986bd9b64bd1eed991b6c9fe88c2f';
const modulePath = '/home/yoshi/git/busnes.app/KyDrive-server/.claude/worktrees/drive-eight/web/node_modules/@playwright/test/index.mjs';
const packageRoot = modulePath.slice(0, modulePath.indexOf('/node_modules/'));
const privateDir = process.env.KYDNS_CAPTURE_PRIVATE;
assert(privateDir?.startsWith('/tmp/kydns-dns-eight-'), 'pass the owned fixture directory');
assert.equal((await stat(privateDir)).mode & 0o777, 0o700);
const git = (...args) => execFileSync('git', ['-C', root, ...args], { encoding: 'utf8' }).trim();
const sha = b => createHash('sha256').update(b).digest('hex');
assert.equal(git('rev-parse', 'HEAD'), frozen);
assert.equal(git('diff', 'HEAD', '--name-only'), '', 'tracked files must stay frozen');
const processRecord = JSON.parse(await readFile(join(privateDir, 'process.json'), 'utf8'));
const proc = `/proc/${processRecord.pid}`;
assert.equal((await readFile(`${proc}/stat`, 'utf8')).split(' ')[21], processRecord.startticks);
assert.equal(sha(await readFile(`${proc}/exe`)), processRecord.binary_sha256);
const buildState = JSON.parse(await readFile(join(privateDir, 'build-state-r4.json'), 'utf8'));
assert.equal(Object.keys(buildState.source_files).length, 85);
for (const [file, hash] of Object.entries(buildState.source_files)) assert.equal(sha(await readFile(join(root, file))), hash, file);
const queryRecord = JSON.parse(await readFile(join(privateDir, 'queries-final-r4.json'), 'utf8'));
assert.equal(queryRecord.length, 24);
assert.equal(queryRecord.filter(q => q.rcode === 0).length, 18);
assert.equal(queryRecord.filter(q => q.rcode === 3).length, 6);
const password = (await readFile(join(privateDir, 'password'), 'utf8')).trim();
const baseURL = 'http://127.0.0.1:5880';
const surfaces = [
  ['login', '/login', 'Sign in'], ['dashboard', '/', 'Dashboard'],
  ['services', '/services', 'Services'], ['records', '/records', 'Records'],
  ['discovered', '/discovered', 'Discovered'], ['blacklists', '/blacklists', 'Blacklists'],
  ['dhcp', '/dhcp', 'DHCP'], ['settings', '/settings', 'Settings'],
  ['appearance', '/appearance', 'Appearance'],
];
const out = join(here, 'screenshots');
await mkdir(out, { recursive: true });
for (const theme of ['light', 'dark']) for (const device of ['desktop', 'mobile']) {
  for (const [surface] of surfaces) {
    try { await access(join(out, `kydns-03-after-${surface}-${device}-${theme}.png`)); }
    catch (e) { if (e.code === 'ENOENT') continue; throw e; }
    throw new Error('Refusing to overwrite original evidence');
  }
}
for (const file of ['manifest-03-after.json', 'capture-provenance-r4.json']) {
  try { await access(join(here, file)); } catch (e) { if (e.code === 'ENOENT') continue; throw e; }
  throw new Error(`Refusing to overwrite ${file}`);
}
async function atomic(file, value) {
  const target = join(here, file), temp = target + '.tmp';
  await writeFile(temp, JSON.stringify(value, null, 2) + '\n', { flag: 'wx' });
  await access(target).then(() => { throw new Error(`Refusing to overwrite ${file}`); }, e => { if (e.code !== 'ENOENT') throw e; });
  await rename(temp, target);
}
const { chromium } = await import(modulePath);
const browser = await chromium.launch({ headless: true });
const started = new Date().toISOString();
const shots = [], assets = new Map(), checks = [];
const safe = s => String(s).split(password).join('[private credential]');
try {
  assert.equal(browser.version(), '156.0.8078.4');
  for (const theme of ['light', 'dark']) {
    for (const [device, viewport] of [['desktop', { width: 1280, height: 900 }], ['mobile', { width: 390, height: 844 }]]) {
      const ctx = await browser.newContext({ baseURL, viewport, deviceScaleFactor: 1, colorScheme: theme,
        locale: 'en-US', timezoneId: 'America/New_York', serviceWorkers: 'allow' });
      const page = await ctx.newPage();
      const errors = [], badHTTP = [], failed = [], external = [], pending = [];
      page.on('console', m => { if (m.type() === 'error') errors.push(safe(m.text())); });
      page.on('pageerror', e => errors.push(safe(e.message)));
      page.on('requestfailed', r => failed.push({ url: r.url(), error: r.failure()?.errorText }));
      page.on('request', r => { if (!r.url().startsWith(baseURL + '/')) external.push(r.url()); });
      page.on('response', r => {
        if (r.status() >= 400) badHTTP.push({ url: r.url(), status: r.status() });
        const path = new URL(r.url()).pathname;
        if (path.startsWith('/static/') && r.status() === 200) pending.push((async () => {
          const bytes = await r.body();
          const file = 'internal/web' + decodeURIComponent(path);
          const sourceHash = sha(await readFile(join(root, file)));
          assert.equal(sha(bytes), sourceHash, `served bytes mismatch: ${path}`);
          assets.set(path, { path, file, sha256: sourceHash, bytes: bytes.length, matches_frozen_worktree: true });
        })());
      });
      let authenticated = false;
      for (const [surface, path, heading] of surfaces) {
        const offsets = [errors.length, badHTTP.length, failed.length];
        const response = await page.goto(path, { waitUntil: 'load' });
        assert.equal(response.status(), 200);
        assert.equal(new URL(page.url()).pathname, path);
        await page.getByRole('heading', { name: heading, exact: true }).first().waitFor();
        if (surface === 'dashboard') await page.locator('#chart-queries svg text').first().waitFor();
        if (surface === 'appearance') await page.locator('[data-theme-picker] input[type=radio]').nth(22).waitFor({ state: 'attached' });
        await page.evaluate(() => document.fonts.ready);
        await page.mouse.move(0, 0);
        await page.waitForTimeout(500);
        await Promise.all(pending.splice(0));
        const state = await page.evaluate(() => {
          const bounds = el => { const r = el.getBoundingClientRect(); return { x: r.x, y: r.y, width: r.width, height: r.height }; };
          const nav = document.querySelector('.app-nav-links');
          return { path: location.pathname, title: document.title, dpr: devicePixelRatio,
            locale: navigator.language, timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
            os_dark: matchMedia('(prefers-color-scheme: dark)').matches,
            theme: document.documentElement.dataset.theme, saved_choice: localStorage.getItem('kydns-theme'),
            scroll: { x: scrollX, y: scrollY, document_width: document.documentElement.scrollWidth, viewport_width: innerWidth,
              document_height: document.documentElement.scrollHeight, viewport_height: innerHeight },
            horizontal_overflow_px: Math.max(0, document.documentElement.scrollWidth - innerWidth),
            internal_horizontal_scrollers: [...document.querySelectorAll('nav,main *')].filter(e =>
              e.scrollWidth > e.clientWidth + 1 && /auto|scroll/.test(getComputedStyle(e).overflowX)).map(e =>
              ({ tag: e.tagName, class: e.className, scrollWidth: e.scrollWidth, clientWidth: e.clientWidth, scrollLeft: e.scrollLeft })),
            hovered_controls: [...document.querySelectorAll('a:hover,button:hover,input:hover,select:hover')].map(e => e.tagName + ':' + (e.id || e.textContent.trim())),
            active_element: { tag: document.activeElement.tagName, id: document.activeElement.id },
            password_fields: document.querySelectorAll('input[type=password]').length,
            nonempty_password_fields: [...document.querySelectorAll('input[type=password]')].filter(e => e.value).length,
            setup_token_fields: document.querySelectorAll('#token').length,
            nav_regions: document.querySelectorAll('nav').length,
            nav: nav ? { bounds: bounds(document.querySelector('nav')), scroll_left: nav.scrollLeft,
              links: [...nav.querySelectorAll('a')].map(e => ({ label: e.textContent.trim(), href: e.getAttribute('href'), current: e.getAttribute('aria-current'), bounds: bounds(e) })) } : null,
            theme_cards: document.querySelectorAll('[data-theme-picker] input[type=radio]').length,
            checked_theme: document.querySelector('[data-theme-picker] input:checked')?.value ?? null,
            headings: [...document.querySelectorAll('main h1,main h2,main h3')].map(e => ({ text: e.textContent.trim(), bounds: bounds(e) })),
            fonts: { space_grotesk: document.fonts.check('15px "Space Grotesk"'), ibm_plex_mono: document.fonts.check('13px "IBM Plex Mono"') },
            routing_disclosures: [...document.querySelectorAll('details')].map(e => ({ label: e.querySelector('summary')?.textContent, open: e.open })),
            fieldsets: [...document.querySelectorAll('fieldset')].map(e => ({ bounds: bounds(e), client_width: e.clientWidth, scroll_width: e.scrollWidth })),
            data_rows: [...document.querySelectorAll('table.data-list tbody tr:not(.column-headings)')].map(row => ({ bounds: bounds(row),
              cells: [...row.querySelectorAll('td')].map(e => { const cs = getComputedStyle(e), label = getComputedStyle(e, '::before');
                const range = document.createRange();
                const firstText = [...e.childNodes].find(n => n.nodeType === Node.TEXT_NODE && n.textContent.trim());
                if (firstText) range.selectNodeContents(firstText);
                return { label: e.dataset.label, bounds: bounds(e), font_size: cs.fontSize, line_height: cs.lineHeight,
                  font_family: cs.fontFamily, display: cs.display, flex_direction: cs.flexDirection, gap: cs.gap,
                  label_style: { content: label.content, display: label.display, font_size: label.fontSize, line_height: label.lineHeight },
                  // ::before has no DOM rect. Its line top follows the flex column content edge.
                  label_top: e.dataset.label && !['Name','Actions'].includes(e.dataset.label) ? bounds(e).y + parseFloat(cs.paddingTop) + parseFloat(cs.borderTopWidth) : null,
                  text_rects: firstText ? [...range.getClientRects()].map(r => ({ x:r.x,y:r.y,width:r.width,height:r.height })) : [],
                  targets: [...e.querySelectorAll('button,summary')].map(t => ({ text:t.textContent.trim(),bounds:bounds(t),min_height:getComputedStyle(t).minHeight })) };
              }) })),
            source_urls: [...document.querySelectorAll('.source-url')].map(e => ({ text: e.textContent, wbr_count: e.querySelectorAll('wbr').length,
              bounds: bounds(e), client_width: e.clientWidth, scroll_width: e.scrollWidth,
              overflow_wrap: getComputedStyle(e).overflowWrap, word_break: getComputedStyle(e).wordBreak,
              segments: [...e.childNodes].filter(n => n.nodeType === Node.TEXT_NODE && n.textContent).map(n => {
                const r = document.createRange(); r.selectNodeContents(n); return { text: n.textContent,
                  rects: [...r.getClientRects()].map(r => ({x:r.x,y:r.y,width:r.width,height:r.height})) };
              }) })),
            query_totals: Object.fromEntries([...document.querySelectorAll('[data-stat]')].map(e => [e.dataset.stat, e.textContent])),
            list_headers: [...document.querySelectorAll('main header a[href^="#"]')].map(e => ({ text: e.textContent, href: e.getAttribute('href'), color: getComputedStyle(e).color, ink: getComputedStyle(document.body).getPropertyValue('--ink').trim(), background: getComputedStyle(e).backgroundColor, bounds: bounds(e), target_exists: !!document.querySelector(e.getAttribute('href')) })),
            settings_save: [...document.querySelectorAll('button')].filter(e => e.textContent.trim() === 'Save settings').map(e => ({ bounds: bounds(e), form_id: e.form?.id, action: e.form ? new URL(e.form.action).pathname : null })),
            chart_ready: !!document.querySelector('#chart-queries svg text'),
            chart_legend: document.querySelector('#chart-queries figcaption')?.innerText ?? null,
            visible_errors: [...document.querySelectorAll('p.error,.banner')].map(e => e.innerText),
          };
        });
        assert.equal(state.dpr, 1); assert.equal(state.saved_choice, null);
        assert.equal(state.theme, `busnes-${theme}`); assert.equal(state.nonempty_password_fields, 0);
        assert.equal(state.setup_token_fields, 0); assert.equal(state.scroll.y, 0); assert.equal(state.scroll.x, 0);
        assert.equal(state.hovered_controls.length, 0);
        assert.equal(state.routing_disclosures.filter(d => d.open).length, 0);
        assert.equal(state.horizontal_overflow_px, 0);
        if (surface !== 'login') assert.equal(state.password_fields === 0 || surface === 'settings', true);
        if (surface === 'appearance') { assert.equal(state.theme_cards, 23); assert.equal(state.checked_theme, 'system'); }
        assert.equal(external.length, 0);
        assert.equal(errors.length, 0, JSON.stringify(errors));
        assert.equal(badHTTP.length, 0, JSON.stringify(badHTTP));
        assert.equal(failed.length, 0, JSON.stringify(failed));
        if (surface !== 'login') assert.equal(state.nav_regions, 1);
        if (surface === 'dashboard') {
          assert.equal(state.query_totals.total, '24'); assert.equal(state.query_totals.blocked, '6');
          assert.equal(state.query_totals.authoritative, '18'); assert.equal(state.query_totals.forwarded, '0');
        }
        if (['services', 'records'].includes(surface)) {
          assert.equal(state.data_rows.length, 3);
          for (const row of state.data_rows) {
            for (const cell of row.cells) {
              for (const t of cell.targets.filter(t => t.bounds.height > 0)) assert.ok(t.bounds.height >= 40, `${surface} ${t.text} target below40`);
              if (cell.label !== 'Name') assert.equal(cell.font_size, '13px');
            }
            if (device === 'mobile') {
              const cell = label => row.cells.find(c => c.label === label);
              const pairs = surface === 'services' ? [['Address','View'],['Aliases','Routing']] : [['Type','Value']];
              for (const [a,b] of pairs) assert.equal(cell(a).label_top, cell(b).label_top, `staggered ${a}/${b}`);
              assert.equal(cell('Name').bounds.y + cell('Name').bounds.height / 2, cell('Actions').bounds.y + cell('Actions').bounds.height / 2);
              for (const c of row.cells.filter(c => !['Name','Actions'].includes(c.label))) {
                assert.equal(c.flex_direction, 'column'); assert.equal(c.label_style.display, 'block');
              }
            }
          }
        }
        if (surface === 'blacklists') {
          assert.equal(state.source_urls.length, 1);
          const u = state.source_urls[0];
          assert.equal(u.text, 'https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts');
          assert.equal(u.wbr_count, 7); assert.equal(u.overflow_wrap, 'break-word');
          assert.ok(u.scroll_width <= u.client_width + 1);
          assert.equal(u.segments.map(s => s.text).join(''), u.text);
          assert.equal(u.segments.find(s => s.text === 'StevenBlack/').rects.length, 1);
        }
        const file = `kydns-03-after-${surface}-${device}-${theme}.png`;
        const bytes = await page.screenshot({ fullPage: false, type: 'png' });
        assert.equal(bytes.readUInt32BE(16), viewport.width); assert.equal(bytes.readUInt32BE(20), viewport.height);
        await writeFile(join(out, file), bytes, { flag: 'wx' });
        shots.push({ file: 'screenshots/' + file, surface, path, device, theme,
          account: surface === 'login' ? 'signed-out' : 'local-recovery-administrator',
          sha256: sha(bytes), bytes: bytes.length, width: viewport.width, height: viewport.height,
          captured_at: new Date().toISOString(), state, errors: errors.slice(offsets[0]), http_ge_400: badHTTP.slice(offsets[1]), request_failures: failed.slice(offsets[2]) });
        console.log(`captured ${surface} ${device} ${theme}`);
        if (!authenticated) {
          await page.locator('#password').fill(password);
          await page.getByRole('button', { name: 'Sign in', exact: true }).click();
          await page.waitForURL(baseURL + '/');
          await page.locator('nav').waitFor();
          assert.equal(await page.locator('input[type=password],#token').count(), 0);
          authenticated = true;
        }
      }
      const rep = await page.goto('/replication');
      const redirectRequest = rep.request().redirectedFrom();
      const redirectResponse = redirectRequest ? await redirectRequest.response() : null;
      assert.equal(redirectResponse?.status(), 303);
      assert.equal(new URL(page.url()).pathname, '/');
      assert.equal(await page.locator('nav a[href="/replication"]').count(), 0);
      checks.push({ device, theme, requested: '/replication', final_path: new URL(page.url()).pathname,
        initial_status: redirectResponse?.status() ?? rep.status(), final_status: rep.status(),
        nav_present: await page.locator('nav a[href="/replication"]').count(), result: 'N/A: standalone hides Replication and redirects to Dashboard' });
      await Promise.all(pending.splice(0));
      assert.equal(external.length, 0);
      checks.push({ device, theme, console_errors: errors, http_ge_400: badHTTP, request_failures: failed, external_requests: external });
      await ctx.close();
    }
  }
  assert.equal(assets.size, 15);
  assert.equal(shots.length, 36);
  assert.equal(new Set(shots.map(s => `${s.surface}/${s.device}/${s.theme}`)).size, 36);
  assert.equal(git('diff', 'HEAD', '--name-only'), '');
  await atomic('manifest-03-after.json', { product: 'KyDNS', stage: '03', phase: 'after',
    frozen_head: frozen, product_source: frozen, started_at: started, completed_at: new Date().toISOString(),
    matrix: { viewports: [[1280, 900], [390, 844]], dpr: 1, schemes: ['light', 'dark'], locale: 'en-US', timezone: 'America/New_York',
      full_page: false, service_workers: 'allow', saved_kydns_theme: null, settling_ms: 500, pointer: [0, 0] },
    count: 36, unavailable: [{ surface: 'replication', path: '/replication', reason: 'Standalone role hides nav and sends 303 to /. No peers or pairing attempted.', excluded_combinations: 4 }],
    fixture: 'fixture-round4.json', capture_provenance: 'capture-provenance-r4.json', source_provenance: 'source-provenance-r4.json',
    limits: ['Initial viewport only. Below-fold controls are unscored.', 'Live ages, uptime, minute positions and latency drift naturally.', 'No quality scores, acceptance, exhaustive behavior, accessibility, integration or production claims.'],
    shots, checks });
  const playwrightPackage = JSON.parse(await readFile(join(dirname(modulePath), 'package.json'), 'utf8'));
  const native = JSON.parse(await readFile(join(privateDir, 'native-probes-r4.json'), 'utf8'));
  await atomic('capture-provenance-r4.json', { driver: { file: 'capture-round4.mjs', sha256: sha(await readFile(fileURLToPath(import.meta.url))) },
    runtime: { node: process.version, os: type(), release: release(), platform: process.platform, arch: process.arch,
      playwright_module: modulePath, playwright: playwrightPackage.version, browser: browser.version(),
      chromium_executable: chromium.executablePath(), chromium_sha256: sha(await readFile(chromium.executablePath())),
      package_json_sha256: sha(await readFile(join(packageRoot, 'package.json'))),
      package_lock_sha256: sha(await readFile(join(packageRoot, 'package-lock.json'))), headless: true },
    auth: 'Real browser form password login in each fresh scheme/viewport context; no storageState, cookies, storage/auth injection or security bypass.',
    interception: false, dom_style_content_mutation: false, native_closed_before_formal: native.closed_at < started,
    native_probe: native, served_assets: [...assets.values()].sort((a, b) => a.path.localeCompare(b.path)),
    vcs_status_at_capture: git('status', '--short'),
    caveats: ['Actual build info and dirty state are preserved in source-provenance-r4.json; untracked reporting evidence is not a product-source edit.',
      'The native inspection ran at DPR2 and is separate from these DPR1 originals.',
      'Browser controls/forms retain shipped defaults, focus and overflow. No image correction or confirmation run.',
      'Server restarted by parent before this task; same persisted fixture, exactly24baseline DNS questions reissued once. Runtime history, uptime, ages and latency drift naturally.'] });
} finally { await browser.close(); }
