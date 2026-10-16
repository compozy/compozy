import { useEffect, useRef, type ReactElement } from "react";

import { Dialog, DialogContent, DialogTitle, DialogTrigger } from "@compozy/ui";

interface SessionAttachmentImagePreviewProps {
  filename: string;
  source: string | File;
  trigger: ReactElement;
}

/** Owns the mounted viewer image and releases file-backed preview URLs on close. */
function PreviewImage({
  source,
  filename,
}: Pick<SessionAttachmentImagePreviewProps, "source" | "filename">) {
  const imageRef = useRef<HTMLImageElement>(null);

  useEffect(() => {
    const image = imageRef.current;
    if (!image || typeof source === "string") return;
    // oxlint-disable-next-line react-doctor/no-create-object-url-without-revoke -- this mounted preview owns and revokes the URL on close or source replacement.
    const url = URL.createObjectURL(source);
    image.src = url;
    return () => URL.revokeObjectURL(url);
  }, [source]);

  return (
    <img
      ref={imageRef}
      src={typeof source === "string" ? source : undefined}
      alt={filename}
      className="min-h-0 max-h-[70dvh] w-full object-contain"
    />
  );
}

/** Keeps attachment bytes inside the session, including window-scoped desktop overlays. */
export function SessionAttachmentImagePreview({
  filename,
  source,
  trigger,
}: SessionAttachmentImagePreviewProps) {
  const triggerRef = useRef<HTMLButtonElement>(null);

  return (
    <Dialog modal disablePointerDismissal={false}>
      <DialogTrigger ref={triggerRef} render={trigger} />
      <DialogContent
        finalFocus={() => {
          // Explicitly restore every dismissal, including backdrop presses on browsers
          // where Base UI skips automatic return focus without preventScroll support.
          queueMicrotask(() => triggerRef.current?.focus({ preventScroll: true }));
          return false;
        }}
        className="flex max-h-[calc(100dvh-2rem)] flex-col sm:max-w-5xl"
      >
        <DialogTitle className="shrink-0 truncate pr-8">{filename}</DialogTitle>
        <PreviewImage source={source} filename={filename} />
      </DialogContent>
    </Dialog>
  );
}
