/**
 * demo-video encoder — frames + camera.json -> final MP4.
 *
 * Mutating: writes shots/ and the output video inside --dir.
 *
 * Separate from the recorder so framing and encoding can be re-tuned without
 * re-recording — the cached frames and camera.json are all it needs.
 *
 *   node encode.mjs --dir <workdir> [--out demo.mp4]
 *
 * Env knobs: OUT_W OUT_H FPS CRF START END PADDING TARGET_MB
 *   START/END default to the full captured span; trim the settle head and any
 *   dead tail here rather than re-recording.
 *   TARGET_MB, when set, re-encodes at +4 CRF (up to 3 times) until the file
 *   fits — the per-frame crops are cached, so only the final pass repeats.
 */
import { execFile, execFileSync } from "node:child_process";
import { mkdirSync, readFileSync, rmSync, statSync, writeFileSync } from "node:fs";
import path from "node:path";
import { promisify } from "node:util";

const run = promisify(execFile);

const args = process.argv.slice(2);
const argOf = flag => {
  const i = args.indexOf(flag);
  return i >= 0 ? args[i + 1] : undefined;
};
const DIR = argOf("--dir");
if (!DIR) {
  console.error("usage: node encode.mjs --dir <workdir> [--out <name.mp4>]");
  process.exit(2);
}
const VIDEO_DIR = path.resolve(DIR);
const FRAME_DIR = path.join(VIDEO_DIR, "frames");
const SHOT_DIR = path.join(VIDEO_DIR, "shots");
const OUT = path.join(VIDEO_DIR, argOf("--out") ?? "demo.mp4");

const track = JSON.parse(readFileSync(path.join(VIDEO_DIR, "camera.json"), "utf8"));
const { width: FW, height: FH } = track.frameSize;
const span = track.frames.at(-1).t;

const OUT_W = Number(process.env.OUT_W ?? 2560);
const OUT_H = Number(process.env.OUT_H ?? Math.round((OUT_W * FH) / FW / 2) * 2);
const FPS = Number(process.env.FPS ?? 60);
let CRF = Number(process.env.CRF ?? 16);
const START = Number(process.env.START ?? 0);
const END = Number(process.env.END ?? span);
const TARGET_MB = process.env.TARGET_MB ? Number(process.env.TARGET_MB) : null;
/** Breathing room around a focused element, as a share of the framed box. */
const PADDING = Number(process.env.PADDING ?? 0.35);
const CONCURRENCY = 8;

// ---------------------------------------------------------------- camera curve
const easeInOutCubic = t => (t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2);

/** A beat resolves to a target zoom plus the point the frame centres on. */
function resolve(beat) {
  if (!beat.rect) return { zoom: beat.zoom, cx: FW / 2, cy: FH / 2 };
  const cx = beat.rect.x + beat.rect.width / 2;
  const cy = beat.rect.y + beat.rect.height / 2;
  // Never crop tighter than the element itself plus padding, even if the beat
  // asks for a bigger magnification than the element can carry.
  const fit = Math.min(
    FW / (beat.rect.width * (1 + PADDING * 2)),
    FH / (beat.rect.height * (1 + PADDING * 2))
  );
  return { zoom: Math.min(beat.zoom, Math.max(1, fit)), cx, cy };
}

const states = track.beats.map(resolve);

function cameraAt(t) {
  let i = 0;
  while (i + 1 < track.beats.length && track.beats[i + 1].t <= t) i++;
  const beat = track.beats[i];
  const to = states[i];
  if (i === 0) return to;

  const from = states[i - 1];
  const p = beat.ease > 0 ? Math.min(1, (t - beat.t) / (beat.ease / 1000)) : 1;
  const e = easeInOutCubic(Math.max(0, p));
  return {
    zoom: from.zoom + (to.zoom - from.zoom) * e,
    cx: from.cx + (to.cx - from.cx) * e,
    cy: from.cy + (to.cy - from.cy) * e,
  };
}

/** Crop rect in source pixels for a given time, clamped inside the frame. */
function cropAt(t) {
  const { zoom, cx, cy } = cameraAt(t);
  const w = Math.round(FW / zoom / 2) * 2;
  const h = Math.round(FH / zoom / 2) * 2;
  const x = Math.round(Math.min(Math.max(cx - w / 2, 0), FW - w));
  const y = Math.round(Math.min(Math.max(cy - h / 2, 0), FH - h));
  return { w, h, x, y, zoom };
}

// ---------------------------------------------------------------- render crops
rmSync(SHOT_DIR, { recursive: true, force: true });
mkdirSync(SHOT_DIR, { recursive: true });

const shots = track.frames.map((f, i) => ({
  src: path.join(FRAME_DIR, f.file),
  dst: path.join(SHOT_DIR, `s-${String(i).padStart(5, "0")}.png`),
  t: f.t,
  crop: cropAt(f.t),
}));

const maxZoom = Math.max(...shots.map(s => s.crop.zoom));
const tightest = Math.min(...shots.map(s => s.crop.w));
console.log(
  `camera: ${track.beats.length} beats, peak ${maxZoom.toFixed(2)}x ` +
    `(tightest crop ${tightest}px wide -> ${OUT_W}px out` +
    `${tightest < OUT_W ? `, ${(OUT_W / tightest).toFixed(2)}x upscale at peak` : ""})`
);

let done = 0;
async function worker(queue) {
  for (const s of queue) {
    const { w, h, x, y } = s.crop;
    await run("ffmpeg", [
      "-v", "error",
      "-i", s.src,
      "-vf", `crop=${w}:${h}:${x}:${y},scale=${OUT_W}:${OUT_H}:flags=lanczos`,
      "-y", s.dst,
    ]);
    if (++done % 50 === 0) process.stdout.write(`  rendered ${done}/${shots.length}\r`);
  }
}

const lanes = Array.from({ length: CONCURRENCY }, (_, i) =>
  worker(shots.filter((_, j) => j % CONCURRENCY === i))
);
const t0 = Date.now();
await Promise.all(lanes);
console.log(`  rendered ${shots.length} frames in ${((Date.now() - t0) / 1000).toFixed(1)}s`);

// ---------------------------------------------------------------- assemble
const lines = [];
for (let i = 0; i < shots.length; i++) {
  const dur = i < shots.length - 1 ? shots[i + 1].t - shots[i].t : 1 / FPS;
  lines.push(`file '${path.basename(shots[i].dst)}'`, `duration ${Math.max(dur, 1e-4).toFixed(6)}`);
}
lines.push(`file '${path.basename(shots.at(-1).dst)}'`);
const listPath = path.join(SHOT_DIR, "shots.txt");
writeFileSync(listPath, lines.join("\n"));

function encodeOnce(crf) {
  const duration = END - START;
  const fadeOut = Math.max(duration - 0.5, 0.1);
  execFileSync(
    "ffmpeg",
    [
      "-v", "error", "-stats",
      "-f", "concat", "-safe", "0", "-i", listPath,
      "-ss", String(START), "-to", String(END),
      "-vf", `fade=t=in:st=0:d=0.4,fade=t=out:st=${fadeOut.toFixed(2)}:d=0.5`,
      "-r", String(FPS),
      "-fps_mode", "cfr",
      "-c:v", "libx264",
      "-crf", String(crf),
      "-preset", "slow",
      "-pix_fmt", "yuv420p",
      "-movflags", "+faststart",
      "-y", OUT,
    ],
    { stdio: "inherit", cwd: SHOT_DIR }
  );
  return statSync(OUT).size / (1024 * 1024);
}

let sizeMb = encodeOnce(CRF);
if (TARGET_MB) {
  for (let attempt = 0; sizeMb > TARGET_MB && attempt < 3; attempt++) {
    CRF += 4;
    console.log(`  ${sizeMb.toFixed(1)}MB > ${TARGET_MB}MB target — retrying at CRF ${CRF}`);
    sizeMb = encodeOnce(CRF);
  }
  if (sizeMb > TARGET_MB) {
    console.error(
      `warning: still ${sizeMb.toFixed(1)}MB after CRF ${CRF} — shorten the cut (START/END) ` +
        `or lower OUT_W`
    );
  }
}

const probe = execFileSync("ffprobe", [
  "-v", "error",
  "-select_streams", "v:0",
  "-show_entries", "stream=width,height,avg_frame_rate",
  "-show_entries", "format=duration,size,bit_rate",
  "-of", "default=nw=1",
  OUT,
]).toString().trim();
console.log(`\n${OUT}  (${sizeMb.toFixed(1)}MB, CRF ${CRF})\n${probe}`);
