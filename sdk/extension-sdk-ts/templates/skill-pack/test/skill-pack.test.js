import assert from "node:assert/strict";
import { readFileSync, statSync } from "node:fs";
import test from "node:test";

test("skill-pack template ships the manifest and starter skill", () => {
  const manifest = readFileSync(new URL("../extension.toml", import.meta.url), "utf8");
  assert.match(manifest, /capabilities = \["skills.ship"\]/);
  assert.match(manifest, /skills = \["skills\/\*"\]/);

  const skill = statSync(new URL("../skills/announce-run/SKILL.md", import.meta.url));
  assert.equal(skill.isFile(), true);
});
