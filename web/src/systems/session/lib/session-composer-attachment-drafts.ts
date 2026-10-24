import type { SessionAttachment } from "../types";

/**
 * Unsent composer attachments outlive the session runtime that staged them.
 *
 * Switching sessions rebuilds the assistant-ui runtime (its identity is the
 * session), and the composer's attachments live only in that runtime. The text
 * draft survives through the session store; this keeps the attachment half of
 * the draft: the files in composer order, per draft owner, and the daemon
 * upload each one already has, so restoring a draft re-adds the same files
 * through the native adapter without uploading them again.
 *
 * Only files are retained, never preview URLs: each tile creates and revokes
 * its own. Entries are released when the attachment is removed, sent, fails to
 * upload, or its draft is discarded, so retained bytes are exactly the bytes
 * staged in unsent drafts during this app run.
 */
const draftFiles = new Map<string, readonly File[]>();
const draftUploads = new Map<File, { scope: string; upload: Promise<SessionAttachment> }>();
/** Files sent or being removed: the composer may still list them until the adapter settles. */
const releasedFiles = new WeakSet<File>();

function uploadScope(workspaceId: string, sessionId: string): string {
  return `${workspaceId}\u0000${sessionId}`;
}

/** Records the files the composer of `draftId` currently holds, in order. */
export function recordComposerDraftAttachments(draftId: string, files: readonly File[]): void {
  const kept = files.filter(file => !releasedFiles.has(file));
  if (kept.length === 0) {
    draftFiles.delete(draftId);
    return;
  }
  draftFiles.set(draftId, kept);
}

/** The files a rebuilt composer of `draftId` should take back, in order. */
export function composerDraftAttachmentFiles(draftId: string): readonly File[] {
  return draftFiles.get(draftId) ?? [];
}

/** Drops a draft's retained files and the uploads they still held. */
export function discardComposerDraftAttachments(draftId: string): void {
  for (const file of draftFiles.get(draftId) ?? []) draftUploads.delete(file);
  draftFiles.delete(draftId);
}

/** Remembers the daemon upload of a draft file; a failed upload is forgotten. */
export function retainComposerDraftUpload(
  workspaceId: string,
  sessionId: string,
  file: File,
  upload: Promise<SessionAttachment>
): void {
  releasedFiles.delete(file);
  draftUploads.set(file, { scope: uploadScope(workspaceId, sessionId), upload });
  upload.catch(() => {
    if (draftUploads.get(file)?.upload === upload) draftUploads.delete(file);
  });
}

/** The upload already made (or in flight) for this file in this session, if any. */
export function retainedComposerDraftUpload(
  workspaceId: string,
  sessionId: string,
  file: File
): Promise<SessionAttachment> | undefined {
  const retained = draftUploads.get(file);
  if (retained?.scope !== uploadScope(workspaceId, sessionId)) return undefined;
  return retained.upload;
}

/** The file left the draft (sent or removed): it is not restored, and a later add uploads anew. */
export function releaseComposerDraftFile(file: File): void {
  draftUploads.delete(file);
  releasedFiles.add(file);
  for (const [draftId, files] of draftFiles) {
    if (files.includes(file)) recordComposerDraftAttachments(draftId, files);
  }
}
