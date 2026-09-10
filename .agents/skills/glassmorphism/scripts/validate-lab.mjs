// Validation harness for assets/material-lab.html (optional; not a runtime dependency of the skill).
// Run from any scratch folder:
//   npm i playwright@1.63 pngjs@7 && npx playwright install chromium-headless-shell
//   LAB=/abs/path/to/material-lab.html OUT=./validation-run.md node validate-lab.mjs
// Checks: console errors, computed material per preset, measured composite contrast behind the widget title
// for every backdrop × material (text hidden, region captured, luminance percentiles), manual solid mode,
// forced-colors / reduced-motion / reduced-transparency / print emulation, keyboard focus and activation,
// 24px targets, 390px and 320px reflow. Backdrop luminance percentiles (for the lab's estimate) come from
// the second block at the bottom.
import { chromium } from 'playwright';
import { PNG } from 'pngjs';
import fs from 'node:fs';
const LAB = 'file://' + process.env.LAB;
const out = [];
const log = (s) => { out.push(s); console.log(s); };
const lum = (r, g, b) => { const f = (c) => { c /= 255; return c <= .03928 ? c / 12.92 : Math.pow((c + .055) / 1.055, 2.4); }; return .2126 * f(r) + .7152 * f(g) + .0722 * f(b); };
const ratio = (l1, l2) => (Math.max(l1, l2) + .05) / (Math.min(l1, l2) + .05);
const parseInk = (s) => (s.match(/[\d.]+/g) || []).map(Number);
const setRadio = (name, value) => page.evaluate(([n, v]) => { const i = document.querySelector(`input[name="${n}"][value="${v}"]`); i.checked = true; i.dispatchEvent(new Event('change', { bubbles: true })); }, [name, value]);
if (!process.argv.includes('--lum')) {
const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 1100 }, deviceScaleFactor: 1 });
const page = await ctx.newPage();
const errors = [];
page.on('console', (m) => { if (m.type() === 'error' || m.type() === 'warning') errors.push(`${m.type()}: ${m.text()}`); });
page.on('pageerror', (e) => errors.push('pageerror: ' + e.message));

await page.goto(LAB); await page.waitForTimeout(600);
log('## Console'); log(errors.length ? errors.join('\n') : 'no console errors or warnings');

// Computed backdrop-filter per material and lens application
log('\n## Computed material per preset');
for (const m of ['clear', 'regular', 'frosted']) {
  await setRadio('material', m); await page.waitForTimeout(150);
  const r = await page.evaluate(() => { const cs = getComputedStyle(document.getElementById('sample')); const play = getComputedStyle(document.getElementById('play')); return { bf: cs.backdropFilter || cs.webkitBackdropFilter, bg: cs.backgroundColor, ink: cs.color, playBf: (play.backdropFilter || '').slice(0, 60) }; });
  log(`- ${m}: sample backdrop-filter=\`${r.bf}\` fill=\`${r.bg}\` ink=\`${r.ink}\`; play button=\`${r.playBf}…\``);
}
await setRadio('material', 'regular');

// Contrast: rendered composite behind the sample title, per backdrop (text made transparent, then sampled)
log('\n## Measured composite behind text (title box of the lead card; worst 5th percentile vs ink)');
const backdrops = ['sunset', 'meadow', 'poppy', 'deep', 'studio', 'pastel', 'stress'];
for (const b of backdrops) {
  await setRadio('backdrop', b); await page.waitForTimeout(250);
  for (const m of ['clear', 'regular', 'frosted']) {
    await setRadio('material', m); await page.waitForTimeout(150);
    const info = await page.evaluate(() => { const h = document.getElementById('w1'); const p = h.parentElement.querySelector('.sub'); const r1 = h.getBoundingClientRect(); const r2 = p.getBoundingClientRect(); const ink = getComputedStyle(h).color; const inkMuted = getComputedStyle(p).color; return { title: [r1.x, r1.y, r1.width, r1.height], sub: [r2.x, r2.y, r2.width, r2.height], ink, inkMuted }; });
    await page.evaluate(() => { document.getElementById('sample').style.setProperty('color', 'transparent'); document.getElementById('sample').querySelectorAll('*').forEach((e) => (e.style.color = 'transparent')); });
    await page.waitForTimeout(80);
    const buf = await page.screenshot({ clip: { x: info.title[0], y: info.title[1], width: info.title[2], height: info.title[3] + info.sub[3] + 6 } });
    await page.evaluate(() => { document.getElementById('sample').style.removeProperty('color'); document.getElementById('sample').querySelectorAll('*').forEach((e) => e.style.removeProperty('color')); });
    const png = PNG.sync.read(buf); const ls = [];
    for (let i = 0; i < png.data.length; i += 4) ls.push(lum(png.data[i], png.data[i + 1], png.data[i + 2]));
    ls.sort((a, b) => a - b);
    const ink = parseInk(info.ink); const li = lum(ink[0], ink[1], ink[2]);
    const mi = parseInk(info.inkMuted); const ma = mi.length > 3 ? mi[3] : 1; const median = ls[Math.floor(ls.length / 2)];
    const lm = ma * lum(mi[0], mi[1], mi[2]) + (1 - ma) * median; // muted ink alpha-blended over the composite
    const worst = li > .5 ? ls[Math.floor(ls.length * .95)] : ls[Math.floor(ls.length * .05)];
    log(`- ${b} / ${m}: title ${ratio(li, median).toFixed(2)}:1 (worst-5% ${ratio(li, worst).toFixed(2)}:1) · secondary ${ratio(lm, median).toFixed(2)}:1`);
  }
}
await setRadio('backdrop', 'sunset'); await setRadio('material', 'regular');

// Manual solid mode
log('\n## Manual solid mode');
await page.click('#solid-toggle'); await page.waitForTimeout(150);
log('- ' + JSON.stringify(await page.evaluate(() => { const cs = getComputedStyle(document.getElementById('sample')); const b = getComputedStyle(document.getElementById('sample'), '::before'); return { bf: cs.backdropFilter, bg: cs.backgroundColor, ink: cs.color, rimPseudo: b.content }; })));
await page.click('#solid-toggle');

// Forced colors + reduced motion + reduced transparency emulation
log('\n## Media emulation');
await page.emulateMedia({ forcedColors: 'active' }); await page.waitForTimeout(150);
log('- forced-colors: ' + JSON.stringify(await page.evaluate(() => { const cs = getComputedStyle(document.getElementById('sample')); return { bf: cs.backdropFilter, bg: cs.backgroundColor, ink: cs.color, border: cs.borderTopColor, backdropHidden: getComputedStyle(document.querySelector('.backdrop')).display, status: document.getElementById('status').textContent }; })));
await page.emulateMedia({ forcedColors: 'none', reducedMotion: 'reduce' }); await page.waitForTimeout(100);
log('- reduced-motion: ' + JSON.stringify(await page.evaluate(() => { const cs = getComputedStyle(document.getElementById('sample')); return { transition: cs.transitionDuration, bf: cs.backdropFilter.slice(0, 40) }; })));
await page.emulateMedia({ reducedMotion: 'no-preference' });
const cdp = await ctx.newCDPSession(page);
await cdp.send('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-reduced-transparency', value: 'reduce' }] }); await page.waitForTimeout(150);
log('- prefers-reduced-transparency: ' + JSON.stringify(await page.evaluate(() => { const cs = getComputedStyle(document.getElementById('sample')); return { bf: cs.backdropFilter, bg: cs.backgroundColor, status: document.getElementById('status').textContent }; })));
await cdp.send('Emulation.setEmulatedMedia', { features: [] });
await page.emulateMedia({ media: 'print' }); await page.waitForTimeout(100);
log('- print: ' + JSON.stringify(await page.evaluate(() => { const cs = getComputedStyle(document.getElementById('sample')); return { bg: cs.backgroundColor, ink: cs.color, bf: cs.backdropFilter, deskHidden: getComputedStyle(document.querySelector('.desk')).display }; })));
await page.emulateMedia({ media: 'screen' });

// Keyboard: tab through, count focus-visible outlines, activate a toggle with keyboard
log('\n## Keyboard');
await page.reload(); await page.waitForTimeout(400);
const focusables = await page.evaluate(() => [...document.querySelectorAll('button, input, a, [tabindex="0"]')].filter((e) => !e.disabled && e.offsetParent !== null).length);
let visibleOutlines = 0, stops = 0;
for (let i = 0; i < 40; i++) { await page.keyboard.press('Tab'); stops++; const ok = await page.evaluate(() => { const e = document.activeElement; if (!e || e === document.body) return null; const cs = getComputedStyle(e.matches('.vh') ? e.nextElementSibling : e); return cs.outlineStyle !== 'none' && parseFloat(cs.outlineWidth) >= 2; }); if (ok) visibleOutlines++; }
log(`- ${focusables} focusable controls; after ${stops} Tab presses, ${visibleOutlines} stops showed a ≥2px outline`);
await page.focus('#grain-toggle'); await page.keyboard.press('Enter'); await page.waitForTimeout(100);
log('- Enter on grain toggle → aria-pressed=' + (await page.getAttribute('#grain-toggle', 'aria-pressed')) + ', grain var=' + (await page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('--glass-grain').trim().slice(0, 30))));
await page.focus('#vol'); await page.keyboard.press('ArrowRight'); log('- ArrowRight on slider → value=' + (await page.inputValue('#vol')));
await page.focus('#notif'); await page.keyboard.press('Space'); log('- Space on switch → aria-checked=' + (await page.getAttribute('#notif', 'aria-checked')));

// Targets and reflow
log('\n## Targets and reflow');
const small = await page.evaluate(() => [...document.querySelectorAll('button, a, input[type="range"], .swatch')].filter((e) => e.offsetParent !== null).map((e) => { const r = e.getBoundingClientRect(); return { n: (e.getAttribute('aria-label') || e.id || e.className || e.textContent).toString().trim().slice(0, 24), w: Math.round(r.width), h: Math.round(r.height) }; }).filter((t) => t.w < 24 || t.h < 24));
log('- targets under 24×24 CSS px: ' + (small.length ? JSON.stringify(small) : 'none'));
for (const w of [390, 320]) {
  await page.setViewportSize({ width: w, height: 900 }); await page.waitForTimeout(300);
  const r = await page.evaluate(() => { const dw = document.documentElement.scrollWidth; const over = [...document.querySelectorAll('.glass, .stage, .desk, .scene')].filter((e) => { const b = e.getBoundingClientRect(); return b.right > innerWidth + 1 || b.left < -1; }).map((e) => e.className.toString().slice(0, 30)); return { scrollWidth: dw, innerWidth, over }; });
  log(`- ${w}px: scrollWidth=${r.scrollWidth} (viewport ${r.innerWidth}); elements past the viewport: ${r.over.length ? r.over.join(', ') : 'none'}`);
}
await page.setViewportSize({ width: 1280, height: 900 });
await page.evaluate(() => { document.documentElement.style.zoom = '4'; }); await page.waitForTimeout(300);
log('- 1280px at zoom 4 (≈320px reflow): scrollWidth=' + (await page.evaluate(() => document.documentElement.scrollWidth)) + ' / innerWidth=' + (await page.evaluate(() => innerWidth)));
await browser.close();
fs.writeFileSync(process.env.OUT, out.join('\n') + '\n');
}

// ---- Backdrop luminance per zone (stack / cluster / desk), used to seed `backdropLum` in the lab ----
// Run with: LAB=... node validate-lab.mjs --lum




if (process.argv.includes('--lum')) {
const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1440, height: 1100 } });
await page.goto(LAB); await page.waitForTimeout(500);
await page.addStyleTag({ content: '.stage, .desk { visibility: hidden !important; }' });
const res = {};
for (const b of ['sunset', 'meadow', 'poppy', 'deep', 'studio', 'pastel', 'stress']) {
  await page.evaluate((v) => { const i = document.querySelector(`input[name="backdrop"][value="${v}"]`); i.checked = true; i.dispatchEvent(new Event('change', { bubbles: true })); }, b);
  await page.waitForTimeout(250);
  const box = await page.evaluate(() => { const r = document.querySelector('.scene').getBoundingClientRect(); return { x: r.x, y: r.y, w: r.width, h: r.height }; });
  const zones = { stack: [0.02, 0.08, 0.27, 0.8], cluster: [0.3, 0.08, 0.7, 0.5], desk: [0.74, 0.03, 0.98, 0.95] };
  res[b] = {};
  for (const [name, [x0, y0, x1, y1]] of Object.entries(zones)) {
    const buf = await page.screenshot({ clip: { x: box.x + box.w * x0, y: box.y + box.h * y0, width: box.w * (x1 - x0), height: box.h * (y1 - y0) } });
    const png = PNG.sync.read(buf); const ls = [];
    for (let i = 0; i < png.data.length; i += 16) ls.push(lum(png.data[i], png.data[i + 1], png.data[i + 2]));
    ls.sort((a, b) => a - b);
    const q = (p) => +ls[Math.floor(ls.length * p)].toFixed(3);
    res[b][name] = { p10: q(.1), median: q(.5), p90: q(.9) };
  }
}
await browser.close();
console.log(JSON.stringify(res, null, 1));
}
