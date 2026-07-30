import { useLayoutEffect, useRef, type RefObject } from "react";

interface TranscriptAnchor {
  messageId: string;
  offsetTop: number;
}

export function useOlderTranscriptAnchor({
  viewportRef,
  isFetchingOlder,
  messageCount,
  loadOlder,
}: {
  viewportRef: RefObject<HTMLDivElement | null>;
  isFetchingOlder: boolean;
  messageCount: number;
  loadOlder: () => void;
}) {
  const anchorRef = useRef<TranscriptAnchor | null>(null);

  const loadOlderWithAnchor = () => {
    const viewport = viewportRef.current;
    if (viewport) {
      const viewportTop = viewport.getBoundingClientRect().top;
      const messageRows = [...viewport.querySelectorAll<HTMLElement>("[data-message-id]")];
      const anchorRow =
        messageRows.find(row => row.getBoundingClientRect().bottom >= viewportTop) ??
        messageRows[0];
      const messageId = anchorRow?.dataset.messageId;
      anchorRef.current =
        anchorRow && messageId
          ? { messageId, offsetTop: anchorRow.getBoundingClientRect().top - viewportTop }
          : null;
    }
    loadOlder();
  };

  useLayoutEffect(() => {
    const viewport = viewportRef.current;
    const anchor = anchorRef.current;
    if (!viewport || !anchor || isFetchingOlder) return;
    const anchorRow = [...viewport.querySelectorAll<HTMLElement>("[data-message-id]")].find(
      row => row.dataset.messageId === anchor.messageId
    );
    if (anchorRow) {
      const nextOffset =
        anchorRow.getBoundingClientRect().top - viewport.getBoundingClientRect().top;
      viewport.scrollTop += nextOffset - anchor.offsetTop;
    }
    anchorRef.current = null;
  }, [isFetchingOlder, messageCount, viewportRef]);

  return loadOlderWithAnchor;
}
