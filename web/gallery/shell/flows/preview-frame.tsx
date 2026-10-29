/**
 * Live preview of a prototype state inside a flow node: the gallery `Frame`, made inert (pointer,
 * wheel and keyboard reach the node, never the prototype) and scaled from a 1440 × 900 page. It
 * trusts only same-origin status messages from its own iframe; no "ready" in 10 s is a failure.
 * `onSettle` reports the end of loading (ready or failed), so the canvas can start the next one.
 */
import { useEffect, useEffectEvent, useReducer, useRef } from "react";

import { Frame } from "#gallery/shell/frame";
import {
  acceptFrameStatus,
  nextPreviewStatus,
  previewTimeoutMs,
  type FrameStatus,
} from "#gallery/shell/frame-status";

const page = { width: 1440, height: 900 };

type PreviewFrameProps = {
  src: string;
  title: string;
  width: number;
  height: number;
  onSettle: () => void;
};

export function PreviewFrame({ src, title, width, height, onSettle }: PreviewFrameProps) {
  const frameRef = useRef<HTMLIFrameElement | null>(null);
  const [status, dispatch] = useReducer(nextPreviewStatus, "loading");

  const settle = useEffectEvent(() => onSettle());
  useEffect(() => {
    // The first accepted status or the timeout ends loading: settle once, then only track errors.
    let settled = false;
    const apply = (event: FrameStatus["status"] | "timeout") => {
      dispatch(event);
      if (settled) return;
      settled = true;
      settle();
    };
    const onMessage = (event: MessageEvent) => {
      const next = acceptFrameStatus(
        event,
        frameRef.current?.contentWindow ?? null,
        window.location.origin
      );
      if (next) apply(next);
    };
    window.addEventListener("message", onMessage);
    const timer = window.setTimeout(() => apply("timeout"), previewTimeoutMs);
    return () => {
      window.removeEventListener("message", onMessage);
      window.clearTimeout(timer);
    };
  }, []);

  const loading = status === "loading";

  if (status === "error") {
    return (
      <p
        className="flex h-full items-center justify-center px-3 text-center text-small-body text-muted"
        data-testid="flow-preview-error"
      >
        Prototype did not load
      </p>
    );
  }

  const scale = width / page.width;
  return (
    <div
      aria-hidden="true"
      className="pointer-events-none relative size-full select-none"
      data-preview-status={status}
      inert
    >
      <Frame
        height={Math.ceil(height / scale)}
        onElement={element => {
          frameRef.current = element;
        }}
        scale={scale}
        src={src}
        tabIndex={-1}
        title={title}
        width={page.width}
      />
      {loading ? (
        <span className="absolute inset-0 flex items-center justify-center bg-canvas text-small-body text-muted">
          Loading…
        </span>
      ) : null}
    </div>
  );
}
