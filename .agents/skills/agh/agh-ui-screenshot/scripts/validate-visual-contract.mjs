// Read-only validator — rejects incomplete or non-passing visual-contract evidence bundles.
// Usage: bun run validate-visual-contract.mjs --bundle <contract-dir>

import { existsSync, readFileSync, statSync } from "node:fs";
import { resolve } from "node:path";
import { PNG } from "pngjs";

const argv = process.argv.slice(2);
let bundlePath;
for (let index = 0; index < argv.length; index += 1) {
  const flag = argv[index];
  if (flag === "--bundle") bundlePath = argv[++index];
  else {
    console.error(`USAGE ERROR: unknown flag ${flag}`);
    process.exit(2);
  }
}
if (!bundlePath) {
  console.error("USAGE ERROR: --bundle <contract-dir> is required");
  process.exit(2);
}

const bundle = resolve(bundlePath);
const requiredFiles = [
  "reference.png",
  "implementation.png",
  "side-by-side.png",
  "diff.png",
  "comparison.json",
  "review.md",
];
const errors = [];

for (const name of requiredFiles) {
  const path = `${bundle}/${name}`;
  if (!existsSync(path)) errors.push(`missing ${name}`);
  else if (!statSync(path).isFile() || statSync(path).size === 0) errors.push(`empty ${name}`);
}

const readPng = name => {
  try {
    return PNG.sync.read(readFileSync(`${bundle}/${name}`));
  } catch (error) {
    errors.push(`invalid ${name}: ${error instanceof Error ? error.message : String(error)}`);
    return undefined;
  }
};

let comparison;
let review = "";
let reference;
let implementation;
if (errors.length === 0) {
  reference = readPng("reference.png");
  implementation = readPng("implementation.png");
  const sideBySide = readPng("side-by-side.png");
  const diff = readPng("diff.png");

  if (reference && implementation) {
    if (reference.width !== implementation.width || reference.height !== implementation.height) {
      errors.push("reference.png and implementation.png dimensions differ");
    }
    if (diff && (diff.width !== reference.width || diff.height !== reference.height)) {
      errors.push("diff.png dimensions do not match the compared pair");
    }
    if (
      sideBySide &&
      (sideBySide.width !== reference.width * 2 + 8 || sideBySide.height !== reference.height)
    ) {
      errors.push("side-by-side.png dimensions do not match the compared pair");
    }
  }

  try {
    comparison = JSON.parse(readFileSync(`${bundle}/comparison.json`, "utf8"));
  } catch (error) {
    errors.push(
      `invalid comparison.json: ${error instanceof Error ? error.message : String(error)}`
    );
  }
  review = readFileSync(`${bundle}/review.md`, "utf8");
}

if (comparison) {
  if (comparison.schemaVersion !== 1) errors.push("comparison.json schemaVersion must be 1");
  if (comparison.reference !== `${bundle}/reference.png`) {
    errors.push("comparison.json reference must point to bundle/reference.png");
  }
  if (comparison.implementation !== `${bundle}/implementation.png`) {
    errors.push("comparison.json implementation must point to bundle/implementation.png");
  }
  if (
    !reference ||
    comparison.width !== reference.width ||
    comparison.height !== reference.height ||
    comparison.totalPixels !== reference.width * reference.height
  ) {
    errors.push("comparison.json dimensions do not match reference.png");
  }
  if (
    !Number.isInteger(comparison.diffPixels) ||
    comparison.diffPixels < 0 ||
    comparison.diffPixels > comparison.totalPixels ||
    !Number.isFinite(comparison.diffRatio) ||
    comparison.diffRatio < 0 ||
    comparison.diffRatio > 1
  ) {
    errors.push("comparison.json diff metric is invalid");
  }
}

const frontmatter = review ? review.match(/^---\n([\s\S]*?)\n---\n/) : undefined;
const field = name => {
  if (!frontmatter) return undefined;
  return frontmatter[1].match(new RegExp(`^${name}:\\s*(.+)$`, "m"))?.[1].trim();
};

if (review) {
  if (!frontmatter) errors.push("review.md is missing YAML frontmatter");
  if (field("schema_version") !== "1") errors.push("review.md schema_version must be 1");
  if (!field("contract_id")) errors.push("review.md contract_id is required");
  if (field("verdict") !== "PASS") errors.push("review.md verdict must be PASS");
  if (field("blocking_divergences") !== "0") {
    errors.push("review.md blocking_divergences must be 0");
  }
  for (const name of [
    "reference_source",
    "reference_source_id",
    "implementation_target",
    "viewport",
    "state",
    "reviewer",
    "reviewed_at",
    "authorized_differences",
  ]) {
    if (!field(name)) errors.push(`review.md ${name} is required`);
  }
  if (!/^[0-9a-f]{40}([0-9a-f]{24})?$/.test(field("reference_source_id") ?? "")) {
    errors.push("review.md reference_source_id must be a git object id");
  }
  if (!/^\d+x\d+$/.test(field("viewport") ?? "")) {
    errors.push("review.md viewport must use <width>x<height>");
  }
  if (Number.isNaN(Date.parse(field("reviewed_at") ?? ""))) {
    errors.push("review.md reviewed_at must be ISO-8601");
  }
  if (!/^\d+$/.test(field("authorized_differences") ?? "")) {
    errors.push("review.md authorized_differences must be a non-negative integer");
  }
  for (const heading of [
    "# Visual Contract Review",
    "## Divergences",
    "## Authorized Differences",
  ]) {
    if (!review.includes(heading)) errors.push(`review.md is missing ${heading}`);
  }
}

if (errors.length > 0) {
  for (const error of errors) console.error(`VISUAL CONTRACT ERROR: ${error}`);
  process.exit(1);
}

console.log(
  `PASS ${field("contract_id")}: ${comparison.diffPixels}/${comparison.totalPixels} diagnostic pixels, 0 blocking divergences`
);
