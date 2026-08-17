/**
 * The session system's attachment surface, gathered in one place: byte URLs,
 * prompt-part extraction, composer ownership hand-off, list-item shaping, and
 * the cards that render them.
 *
 * These belong together because they answer one question — what did the
 * operator attach and how is it shown — and grouping them keeps the public
 * barrel readable as a surface contract rather than a flat list.
 */
export { sessionAttachmentBytesURL, sessionAttachmentIdFromURI } from "./lib/attachment-url";
export { attachmentsFromPromptMessageParts } from "./lib/attachment-kinds";
export {
  consumeSubmittedComposerAttachment,
  retainSubmittedComposerAttachments,
} from "./lib/session-composer-attachment-ownership";
export {
  attachmentExtensionMark,
  formatAttachmentBytes,
  isImageMediaType,
  sessionAttachmentMediaType,
  userMessageAttachmentItems,
  userMessageHasAttachments,
  userMessageHasText,
} from "./lib/session-attachment-items";
export type {
  SessionAttachmentFileItem,
  SessionAttachmentImageItem,
  SessionAttachmentItem,
} from "./lib/session-attachment-items";
export {
  SessionAttachmentFileCard,
  type SessionAttachmentFileCardProps,
} from "./components/session-attachment-file-card";
export {
  SessionAttachmentFrame,
  type SessionAttachmentFrameProps,
} from "./components/session-attachment-frame";
export {
  SessionAttachmentGallery,
  type SessionAttachmentGalleryProps,
} from "./components/session-attachment-gallery";
