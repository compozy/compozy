import assert from "node:assert/strict";
import { readFileSync, statSync } from "node:fs";
import test from "node:test";

test("review provider template ships the expected manifest contract", () => {
  const manifest = readFileSync(new URL("../extension.toml", import.meta.url), "utf8");
  assert.match(manifest, /capabilities = \["providers.register"\]/);
  assert.match(manifest, /\[\[providers.review\]\]/);
  assert.match(manifest, /command = "\.\/bin\/review-provider\.mjs"/);

  const script = statSync(new URL("../bin/review-provider.mjs", import.meta.url));
  assert.equal(script.isFile(), true);
});
