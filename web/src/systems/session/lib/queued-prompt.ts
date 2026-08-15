export interface QueuedPromptAttachmentSummary {
  fileCount: number;
  imageCount: number;
  previewKind?: "image" | "file";
  previewMark?: string;
  previewName?: string;
  previewUrl?: string;
}

export interface QueuedPrompt {
  attachments?: QueuedPromptAttachmentSummary;
  id: string;
  mode?: string;
  status?: string;
  text: string;
}
