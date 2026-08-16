import type { Attachment } from "@assistant-ui/react";

const submittedAttachmentIDs = new Set<string>();

/**
 * Queue and interrupt own the uploaded references after their request succeeds.
 * The composer still clears its local previews, so the adapter must not treat
 * that cleanup as a request to delete the daemon-owned attachment. IDs remain
 * stable even when assistant-ui recreates an attachment object for removal.
 */
export function retainSubmittedComposerAttachments(attachments: readonly Attachment[]): void {
  for (const attachment of attachments) submittedAttachmentIDs.add(attachment.id);
}

export function consumeSubmittedComposerAttachment(attachment: Attachment): boolean {
  if (!submittedAttachmentIDs.has(attachment.id)) return false;
  submittedAttachmentIDs.delete(attachment.id);
  return true;
}
