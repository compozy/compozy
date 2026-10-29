import { startGallery } from "../server.ts";

/** `proto dev`: the gallery on http://127.0.0.1:4400 (strict port). */
export async function dev() {
  const server = await startGallery();
  server.printUrls();
  const stop = () => {
    void server.close().then(() => process.exit(0));
  };
  process.once("SIGINT", stop);
  process.once("SIGTERM", stop);
}
