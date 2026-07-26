import type { WindowManagerLayoutResourceRecord } from "./window-manager-layout-types";

/**
 * A saved-layout record is identified by its resource scope as well as its id:
 * a workspace record may intentionally override a global record with the same
 * id. UI selection and cache replacement must preserve that distinction.
 */
export function layoutProfileResourceKey(record: WindowManagerLayoutResourceRecord): string {
  return `${record.scope.kind}:${record.scope.id}:${record.id}`;
}

export function isSelectedLayoutProfile(
  selectedKey: string | null,
  record: WindowManagerLayoutResourceRecord
): boolean {
  return selectedKey === layoutProfileResourceKey(record);
}
