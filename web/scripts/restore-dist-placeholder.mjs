import { mkdir, writeFile } from "node:fs/promises";

const placeholderPath = new URL("../dist/.keep", import.meta.url);

await mkdir(new URL("../dist/", import.meta.url), { recursive: true });
await writeFile(
  placeholderPath,
  "Tracked placeholder for the daemon web bundle directory.\n",
  "utf8"
);
