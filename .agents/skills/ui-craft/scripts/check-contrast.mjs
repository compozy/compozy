#!/usr/bin/env node
// check-contrast.mjs — verify foreground/background contrast against WCAG 2.2 AA/AAA + APCA Lc.
// Zero-dependency. Supports hex (#RGB / #RRGGBB / #RRGGBBAA), rgb(), oklch().
// Usage:
//   node check-contrast.mjs --fg "#0a0a0a" --bg "#ffffff"
//   node check-contrast.mjs --fg "oklch(20% 0 0)" --bg "oklch(98% 0 0)" --large
//   node check-contrast.mjs --json tokens.json   (file with [{fg, bg, label?, large?}])
// Exit codes: 0 = all pairs pass AA; 1 = any pair fails AA; 2 = bad input.

import { readFileSync } from "node:fs";
import { argv, exit } from "node:process";

function parseArgs(args) {
  const out = { fg: null, bg: null, large: false, json: null, help: false };
  for (let i = 2; i < args.length; i++) {
    const a = args[i];
    if (a === "--fg") out.fg = args[++i];
    else if (a === "--bg") out.bg = args[++i];
    else if (a === "--large") out.large = true;
    else if (a === "--json") out.json = args[++i];
    else if (a === "-h" || a === "--help") out.help = true;
  }
  return out;
}

function help() {
  console.log(`check-contrast.mjs — WCAG AA/AAA + APCA contrast checker.

Usage:
  --fg <color>    Foreground color (hex, rgb(), oklch()).
  --bg <color>    Background color (hex, rgb(), oklch()).
  --large         Treat text as "large" (≥ 18.66px bold or ≥ 24px regular).
  --json <file>   JSON array of {fg, bg, label?, large?} objects.

Examples:
  node check-contrast.mjs --fg "#0a0a0a" --bg "#ffffff"
  node check-contrast.mjs --fg "oklch(20% 0 0)" --bg "oklch(98% 0 0)"
  node check-contrast.mjs --json tokens.json

Exit codes: 0 = all pass AA; 1 = any fail AA; 2 = input error.`);
}

// ---------- Color parsing ----------

function clamp(n, lo, hi) { return Math.max(lo, Math.min(hi, n)); }

function parseHex(s) {
  let h = s.trim().replace(/^#/, "");
  if (h.length === 3) h = h.split("").map(c => c + c).join("");
  if (h.length === 4) h = h.split("").map(c => c + c).join("").slice(0, 8);
  if (!/^[0-9a-f]{6}([0-9a-f]{2})?$/i.test(h)) return null;
  const r = parseInt(h.slice(0, 2), 16);
  const g = parseInt(h.slice(2, 4), 16);
  const b = parseInt(h.slice(4, 6), 16);
  return [r, g, b];
}

function parseRgb(s) {
  const m = s.match(/^rgba?\s*\(\s*([^)]+)\s*\)$/i);
  if (!m) return null;
  const parts = m[1].split(/[,\s/]+/).filter(Boolean).slice(0, 3);
  if (parts.length !== 3) return null;
  const vals = parts.map(p => {
    if (p.endsWith("%")) return Math.round(parseFloat(p) * 2.55);
    return Math.round(parseFloat(p));
  });
  if (vals.some(n => Number.isNaN(n))) return null;
  return vals.map(n => clamp(n, 0, 255));
}

function parseOklch(s) {
  const m = s.match(/^oklch\s*\(\s*([^)]+)\s*\)$/i);
  if (!m) return null;
  const parts = m[1].split(/[,\s/]+/).filter(Boolean).slice(0, 3);
  if (parts.length < 3) return null;
  let L = parts[0].endsWith("%") ? parseFloat(parts[0]) / 100 : parseFloat(parts[0]);
  let C = parseFloat(parts[1]);
  let H = parseFloat(parts[2]);
  if ([L, C, H].some(v => Number.isNaN(v))) return null;
  return oklchToRgb(L, C, H);
}

// OKLCH → linear sRGB → sRGB (8-bit) reference impl
function oklchToRgb(L, C, hDeg) {
  const h = (hDeg * Math.PI) / 180;
  const a = Math.cos(h) * C;
  const b_ = Math.sin(h) * C;
  // Lab (OKLab) → LMS
  const l_ = L + 0.3963377774 * a + 0.2158037573 * b_;
  const m_ = L - 0.1055613458 * a - 0.0638541728 * b_;
  const s_ = L - 0.0894841775 * a - 1.291485548 * b_;
  const l = l_ * l_ * l_;
  const m = m_ * m_ * m_;
  const s = s_ * s_ * s_;
  // LMS → linear sRGB
  let r = +4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s;
  let g = -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s;
  let b = -0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s;
  // Linear → sRGB
  const toSrgb = c => {
    if (c <= 0.0031308) c = 12.92 * c;
    else c = 1.055 * Math.pow(c, 1 / 2.4) - 0.055;
    return clamp(Math.round(c * 255), 0, 255);
  };
  return [toSrgb(r), toSrgb(g), toSrgb(b)];
}

function parseColor(s) {
  if (typeof s !== "string") return null;
  const t = s.trim();
  if (t.startsWith("#")) return parseHex(t);
  if (t.toLowerCase().startsWith("rgb")) return parseRgb(t);
  if (t.toLowerCase().startsWith("oklch")) return parseOklch(t);
  // bare hex without #?
  if (/^[0-9a-f]{3,8}$/i.test(t)) return parseHex(t);
  return null;
}

// ---------- Contrast math ----------

// WCAG 2.x relative luminance
function relativeLuminance([r, g, b]) {
  const chan = c => {
    const cs = c / 255;
    return cs <= 0.03928 ? cs / 12.92 : Math.pow((cs + 0.055) / 1.055, 2.4);
  };
  return 0.2126 * chan(r) + 0.7152 * chan(g) + 0.0722 * chan(b);
}

function wcagContrast(rgb1, rgb2) {
  const L1 = relativeLuminance(rgb1);
  const L2 = relativeLuminance(rgb2);
  const lighter = Math.max(L1, L2);
  const darker = Math.min(L1, L2);
  return (lighter + 0.05) / (darker + 0.05);
}

// APCA Lc — implementation of the public APCA-W3 algorithm (Lc only, no font lookup table).
// Reference: https://github.com/Myndex/apca-w3 (MIT). Compact rewrite.
function apcaLc(textRgb, bgRgb) {
  const sRGBtrc = 2.4;
  const Rco = 0.2126729, Gco = 0.7151522, Bco = 0.0721750;
  const normBG = 0.56, normTXT = 0.57, revTXT = 0.62, revBG = 0.65;
  const blkThrs = 0.022, blkClmp = 1.414;
  const scaleBoW = 1.14, loBoWoffset = 0.027;
  const scaleWoB = 1.14, loWoBoffset = 0.027;
  const deltaYmin = 0.0005;

  const sY = ([r, g, b]) => {
    const f = c => Math.pow(c / 255, sRGBtrc);
    return f(r) * Rco + f(g) * Gco + f(b) * Bco;
  };

  let txtY = sY(textRgb);
  let bgY = sY(bgRgb);

  if (txtY < blkThrs) txtY += Math.pow(blkThrs - txtY, blkClmp);
  if (bgY < blkThrs) bgY += Math.pow(blkThrs - bgY, blkClmp);

  if (Math.abs(bgY - txtY) < deltaYmin) return 0;

  let outputContrast;
  if (bgY > txtY) {
    // light bg, dark text (BoW)
    const SAPC = (Math.pow(bgY, normBG) - Math.pow(txtY, normTXT)) * scaleBoW;
    outputContrast = SAPC < loBoWoffset ? 0 : SAPC - loBoWoffset;
  } else {
    // dark bg, light text (WoB)
    const SAPC = (Math.pow(bgY, revBG) - Math.pow(txtY, revTXT)) * scaleWoB;
    outputContrast = SAPC > -loWoBoffset ? 0 : SAPC + loWoBoffset;
  }

  return outputContrast * 100;
}

// ---------- Verdict ----------

function verdictFor({ fg, bg, label, large }) {
  const ratio = wcagContrast(fg, bg);
  const lc = apcaLc(fg, bg);

  const aaThreshold = large ? 3.0 : 4.5;
  const aaaThreshold = large ? 4.5 : 7.0;

  const passAA = ratio >= aaThreshold;
  const passAAA = ratio >= aaaThreshold;

  return {
    label,
    fg,
    bg,
    large,
    ratio: Math.round(ratio * 100) / 100,
    apcaLc: Math.round(Math.abs(lc) * 10) / 10,
    aa: passAA,
    aaa: passAAA,
  };
}

function formatRgb([r, g, b]) {
  return `#${[r, g, b].map(c => c.toString(16).padStart(2, "0")).join("")}`;
}

function printResult(r) {
  const tag = r.label ? `[${r.label}] ` : "";
  const size = r.large ? "large" : "body";
  const aa = r.aa ? "PASS" : "FAIL";
  const aaa = r.aaa ? "PASS" : "FAIL";
  console.log(
    `${tag}${formatRgb(r.fg)} on ${formatRgb(r.bg)} (${size}) — ` +
    `WCAG ${r.ratio.toFixed(2)}:1 (AA ${aa}, AAA ${aaa}) — APCA Lc ${r.apcaLc.toFixed(1)}`
  );
}

// ---------- Main ----------

function main() {
  const args = parseArgs(argv);
  if (args.help) { help(); exit(0); }

  let pairs = [];

  if (args.json) {
    let raw;
    try {
      raw = JSON.parse(readFileSync(args.json, "utf8"));
    } catch (e) {
      console.error(`Could not read or parse JSON file: ${args.json}`);
      console.error(e.message);
      exit(2);
    }
    if (!Array.isArray(raw)) {
      console.error("JSON input must be an array of {fg, bg, label?, large?} objects.");
      exit(2);
    }
    pairs = raw.map((p, i) => {
      const fg = parseColor(p.fg);
      const bg = parseColor(p.bg);
      if (!fg || !bg) {
        console.error(`Pair #${i}: could not parse color (fg=${p.fg}, bg=${p.bg}).`);
        exit(2);
      }
      return { fg, bg, label: p.label || `pair-${i}`, large: !!p.large };
    });
  } else {
    if (!args.fg || !args.bg) {
      console.error("Provide --fg and --bg, or --json <file>. See --help.");
      exit(2);
    }
    const fg = parseColor(args.fg);
    const bg = parseColor(args.bg);
    if (!fg || !bg) {
      console.error(`Could not parse colors (fg=${args.fg}, bg=${args.bg}).`);
      exit(2);
    }
    pairs = [{ fg, bg, label: null, large: args.large }];
  }

  let anyFail = false;
  for (const p of pairs) {
    const r = verdictFor(p);
    printResult(r);
    if (!r.aa) anyFail = true;
  }

  if (anyFail) {
    console.error("\nAt least one pair fails WCAG AA — see above.");
    exit(1);
  }
  exit(0);
}

main();
