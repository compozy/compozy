import type { PermissionRequest } from "../types";

/** Shared command → resource subject used by both live docks and receipts. */
export function primaryPermissionSubject(permission: PermissionRequest): string | null {
  const command = permission.toolInput.command;
  if (typeof command === "string" && command.trim().length > 0) {
    return command.trim();
  }
  const resource = permission.resource.trim();
  return resource.length > 0 ? resource : null;
}
