import { fileURLToPath } from "node:url";

/** `web/`, the workspace that owns the gallery and every prototype. */
export const webRoot = fileURLToPath(new URL("../../", import.meta.url));
/** The repository root: file paths in problems, entries and source tags are relative to it. */
export const repoRoot = fileURLToPath(new URL("../../../", import.meta.url));
/** `web/gallery/`, the Vite root of the gallery. */
export const galleryRoot = fileURLToPath(new URL("../", import.meta.url));

/** The local dev server: `proto dev` binds it and every printed URL points at it. */
export const galleryHost = "127.0.0.1";
export const galleryPort = 4400;
export const galleryOrigin = `http://${galleryHost}:${galleryPort}`;
