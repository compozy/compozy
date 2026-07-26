/**
 * Compound `(provider, model)` identity for the runtime selector.
 *
 * Two providers can publish the same model id (e.g. an OpenAI-compatible gateway
 * that re-exposes `gpt-5.6-sol` alongside the first-party Codex provider). The
 * emitted wire value stays a separate `{ provider, model }` pair — this key is
 * used ONLY for in-UI identity (selection highlight, list dedupe, favorites, and
 * recents in localStorage) so one row never shadows another.
 *
 * The encoding is a strict, one-way builder: a numeric provider-length prefix
 * means two different `(provider, model)` pairs can never collide onto the same
 * key, even when a field contains the separator character or a custom model id
 * contains arbitrary text. There is deliberately no decoder — the selector never
 * reads a provider or model back out of a key, so no legacy/foreign value can be
 * silently reinterpreted.
 */

/** ASCII unit separator (U+001F) — never part of a provider id or a canonical model id. */
const KEY_SEPARATOR = String.fromCharCode(31);

/** Build the collision-free compound identity for a `(provider, model)` pair. */
export function runtimeModelKey(provider: string, model: string): string {
  return `${provider.length}${KEY_SEPARATOR}${provider}${model}`;
}
