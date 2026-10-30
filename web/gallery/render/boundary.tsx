import { Component, useEffect, type ReactNode } from "react";

import { frameStatusType, postFrameStatus } from "#gallery/shell/frame-status";

/** Marks the document for `proto screenshot`, which waits for a settled prototype. */
function markDocument(status: "ready" | "error") {
  document.documentElement.dataset.prototypeStatus = status;
}

/** Tells the embedding gallery frame (and screenshots) that the prototype committed its render. */
export function ReportReady({ slug, state }: { slug: string; state: string }) {
  useEffect(() => {
    markDocument("ready");
    postFrameStatus({ type: frameStatusType, status: "ready", slug, state });
  }, [slug, state]);
  return null;
}

export function PrototypeError({ message }: { message: string }) {
  return (
    <pre className="p-6 font-mono text-mono-id whitespace-pre-wrap text-danger">{message}</pre>
  );
}

type BoundaryProps = { slug: string; children: ReactNode };

/** A prototype that throws while rendering shows the error and reports it to the gallery. */
export class PrototypeBoundary extends Component<BoundaryProps, { error: string | null }> {
  state = { error: null as string | null };

  static getDerivedStateFromError(error: unknown) {
    return { error: String(error) };
  }

  componentDidCatch(error: unknown) {
    markDocument("error");
    postFrameStatus({
      type: frameStatusType,
      status: "error",
      slug: this.props.slug,
      message: String(error),
    });
  }

  render() {
    if (this.state.error) return <PrototypeError message={this.state.error} />;
    return this.props.children;
  }
}
