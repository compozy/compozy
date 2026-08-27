/**
 * Operator-facing wake text.
 *
 * The daemon's wake is a fact line. Residual model fences, fetch instructions,
 * and XML wrappers are not for the operator — they stay out of the stream.
 */

const UNTRUSTED_BLOCK = /<untrusted-call-result\b[^>]*>[\s\S]*?<\/untrusted-call-result>/gi;
const UNTRUSTED_TAG = /<\/?untrusted-call-result\b[^>]*>/gi;
const FETCH_LINE = /compozy__call_result/i;

export function operatorWakePreview(text: string): string {
  return text
    .replaceAll(UNTRUSTED_BLOCK, "")
    .replaceAll(UNTRUSTED_TAG, "")
    .split("\n")
    .filter(line => !FETCH_LINE.test(line))
    .join("\n")
    .trim();
}
