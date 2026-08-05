/**
 * demo-video GIF export — MP4 -> optimized GIF for inline embeds.
 *
 * Mutating: writes the GIF (and a temp palette) next to the input.
 *
 *   node gif.mjs <in.mp4> [-o <out.gif>]
 *
 * Env knobs: WIDTH (default 960) FPS (default 12) START END
 * Two-pass palette (stats_mode=diff + sierra2_4a dither) — the difference
 * between a muddy 30MB GIF and a crisp one under GitHub's 10MB image limit.
 */
import { execFileSync } from "node:child_process";
import { rmSync, statSync } from "node:fs";
import path from "node:path";

const args = process.argv.slice(2);
const input = args.find(a => !a.startsWith("-"));
if (!input) {
  console.error("usage: node gif.mjs <in.mp4> [-o <out.gif>]");
  process.exit(2);
}
const oIdx = args.indexOf("-o");
const out = oIdx >= 0 ? args[oIdx + 1] : input.replace(/\.[^.]+$/, ".gif");

const WIDTH = Number(process.env.WIDTH ?? 960);
const FPS = Number(process.env.FPS ?? 12);
const START = process.env.START;
const END = process.env.END;

const trim = [...(START ? ["-ss", START] : []), ...(END ? ["-to", END] : [])];
const filters = `fps=${FPS},scale=${WIDTH}:-1:flags=lanczos`;
const palette = path.join(path.dirname(out), ".palette.png");

execFileSync("ffmpeg", [
  "-v", "error",
  ...trim, "-i", input,
  "-vf", `${filters},palettegen=stats_mode=diff`,
  "-y", palette,
]);
execFileSync("ffmpeg", [
  "-v", "error",
  ...trim, "-i", input, "-i", palette,
  "-lavfi", `${filters}[x];[x][1:v]paletteuse=dither=sierra2_4a`,
  "-y", out,
]);
rmSync(palette, { force: true });

const mb = statSync(out).size / (1024 * 1024);
console.log(`${out}  (${mb.toFixed(1)}MB, ${WIDTH}px wide @ ${FPS}fps)`);
if (mb > 10) {
  console.error(
    `warning: ${mb.toFixed(1)}MB exceeds GitHub's 10MB image limit — lower WIDTH/FPS or trim`
  );
}
