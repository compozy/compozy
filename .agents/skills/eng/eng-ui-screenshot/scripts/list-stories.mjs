// Read-only helper — fetches a Storybook server's index.json and emits one story-id per line.
// Required dep in the working dir: none (uses node built-in http).
// Usage:
//   bun run list-stories.mjs <storybook-base-url> [--filter <substring>]
//   e.g. bun run list-stories.mjs http://localhost:6006 --filter systems-tasks-routes
// Output:
//   stdout: one story-id per line (filtered by --filter substring if given).
//   stderr: HTTP / JSON parse errors.
//   exit 0 on success, non-zero on fetch failure.

import http from "node:http";
import https from "node:https";

const argv = process.argv.slice(2);
const baseUrl = argv[0];
if (!baseUrl) {
  console.error("USAGE: list-stories.mjs <storybook-base-url> [--filter <substring>]");
  process.exit(2);
}
let filter = "";
for (let i = 1; i < argv.length; i++) {
  if (argv[i] === "--filter") filter = argv[++i] || "";
}

const indexUrl = `${baseUrl.replace(/\/$/, "")}/index.json`;
const client = indexUrl.startsWith("https:") ? https : http;

const body = await new Promise((resolve, reject) => {
  client
    .get(indexUrl, (res) => {
      if (res.statusCode !== 200) {
        reject(new Error(`HTTP ${res.statusCode} fetching ${indexUrl}`));
        return;
      }
      let buf = "";
      res.on("data", (chunk) => (buf += chunk));
      res.on("end", () => resolve(buf));
      res.on("error", reject);
    })
    .on("error", reject);
});

let data;
try {
  data = JSON.parse(body);
} catch (parseErr) {
  console.error(`JSON parse failed for ${indexUrl}: ${parseErr.message}`);
  process.exit(3);
}

const ids = Object.keys(data.entries || {})
  .filter((id) => data.entries[id].type === "story")
  .filter((id) => (filter ? id.includes(filter) : true))
  .sort();

for (const id of ids) console.log(id);
