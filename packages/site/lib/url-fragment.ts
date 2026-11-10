/**
 * The id a URL fragment names. Fragments are percent-encoded, but anyone can type a malformed one
 * (`#100%`); that names nothing, so it reads back as written instead of throwing.
 */
export function fragmentId(hash: string): string {
  const raw = hash.startsWith("#") ? hash.slice(1) : hash;
  try {
    return decodeURIComponent(raw);
  } catch {
    return raw;
  }
}
