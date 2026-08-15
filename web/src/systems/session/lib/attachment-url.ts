const SESSION_ATTACHMENT_URI_PATTERN = /^compozy:\/\/session-attachments\/([^/?#]+)$/;

export function sessionAttachmentIdFromURI(uri: string): string | null {
  const match = SESSION_ATTACHMENT_URI_PATTERN.exec(uri.trim());
  const attachmentId = match?.[1];
  return attachmentId && attachmentId.length > 0 ? attachmentId : null;
}

/**
 * Resolve a persisted session-attachment URI to the workspace-scoped bytes
 * route. Returns null for anything that is not `compozy://session-attachments/<id>`
 * — including `data:` and external http(s) URLs.
 */
export function sessionAttachmentBytesURL(
  workspaceId: string,
  sessionId: string,
  uri: string
): string | null {
  const attachmentId = sessionAttachmentIdFromURI(uri);
  const workspace = workspaceId.trim();
  const session = sessionId.trim();
  if (!attachmentId || workspace.length === 0 || session.length === 0) {
    return null;
  }
  return `/api/workspaces/${encodeURIComponent(workspace)}/sessions/${encodeURIComponent(session)}/attachments/${encodeURIComponent(attachmentId)}/bytes`;
}
