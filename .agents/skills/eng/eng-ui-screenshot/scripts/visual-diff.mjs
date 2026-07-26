// Mutating evidence helper — reads two same-size PNGs and writes diagnostic artifacts.
// Usage: bun run visual-diff.mjs --reference <png> --implementation <png> --out <dir>

import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import pixelmatch from "pixelmatch";
import { PNG } from "pngjs";

const argv = process.argv.slice(2);
let referencePath;
let implementationPath;
let outDir;
let threshold = 0.1;

for (let index = 0; index < argv.length; index += 1) {
  const flag = argv[index];
  if (flag === "--reference") referencePath = argv[++index];
  else if (flag === "--implementation") implementationPath = argv[++index];
  else if (flag === "--out") outDir = argv[++index];
  else if (flag === "--threshold") threshold = Number(argv[++index]);
  else {
    console.error(`USAGE ERROR: unknown flag ${flag}`);
    process.exit(2);
  }
}

if (!referencePath || !implementationPath || !outDir) {
  console.error("USAGE ERROR: --reference <png> --implementation <png> --out <dir> are required");
  process.exit(2);
}
if (!Number.isFinite(threshold) || threshold < 0 || threshold > 1) {
  console.error("USAGE ERROR: --threshold must be a number from 0 to 1");
  process.exit(2);
}

try {
  const reference = PNG.sync.read(readFileSync(referencePath));
  const implementation = PNG.sync.read(readFileSync(implementationPath));
  if (reference.width !== implementation.width || reference.height !== implementation.height) {
    throw new Error(
      `dimension mismatch: reference ${reference.width}x${reference.height}, ` +
        `implementation ${implementation.width}x${implementation.height}`
    );
  }

  mkdirSync(outDir, { recursive: true });
  const diff = new PNG({ width: reference.width, height: reference.height });
  const diffPixels = pixelmatch(
    reference.data,
    implementation.data,
    diff.data,
    reference.width,
    reference.height,
    {
      threshold,
      includeAA: false,
      alpha: 0.55,
      diffColor: [232, 87, 42],
      aaColor: [142, 142, 181],
    }
  );

  const dividerWidth = 8;
  const sideBySide = new PNG({
    width: reference.width * 2 + dividerWidth,
    height: reference.height,
    fill: true,
  });
  PNG.bitblt(reference, sideBySide, 0, 0, reference.width, reference.height, 0, 0);
  for (let y = 0; y < reference.height; y += 1) {
    for (let x = reference.width; x < reference.width + dividerWidth; x += 1) {
      const offset = (sideBySide.width * y + x) << 2;
      sideBySide.data[offset] = 232;
      sideBySide.data[offset + 1] = 87;
      sideBySide.data[offset + 2] = 42;
      sideBySide.data[offset + 3] = 255;
    }
  }
  PNG.bitblt(
    implementation,
    sideBySide,
    0,
    0,
    implementation.width,
    implementation.height,
    reference.width + dividerWidth,
    0
  );

  const totalPixels = reference.width * reference.height;
  const comparison = {
    schemaVersion: 1,
    reference: resolve(referencePath),
    implementation: resolve(implementationPath),
    sideBySideOrder: ["reference", "implementation"],
    width: reference.width,
    height: reference.height,
    threshold,
    diffPixels,
    totalPixels,
    diffRatio: diffPixels / totalPixels,
    note: "The pixel ratio is diagnostic and never proves semantic visual parity.",
  };

  writeFileSync(`${outDir}/diff.png`, PNG.sync.write(diff));
  writeFileSync(`${outDir}/side-by-side.png`, PNG.sync.write(sideBySide));
  writeFileSync(`${outDir}/comparison.json`, `${JSON.stringify(comparison, null, 2)}\n`);
  console.log(
    `saved visual diff: ${diffPixels}/${totalPixels} pixels (${(comparison.diffRatio * 100).toFixed(
      4
    )}%)`
  );
} catch (error) {
  console.error(`VISUAL DIFF ERROR: ${error instanceof Error ? error.message : String(error)}`);
  process.exit(1);
}
