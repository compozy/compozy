"use client";

import { useEffect, useId, useReducer, useRef } from "react";
import { Eyebrow } from "@agh/ui";

let mermaidLoader: Promise<typeof import("mermaid").default> | null = null;

function loadMermaid() {
  if (!mermaidLoader) {
    mermaidLoader = import("mermaid")
      .then(({ default: mermaid }) => {
        // Mermaid emits SVG with `fill` / `stroke` attributes set to these
        // theme variable values. SVG attributes accept `var(--…)`, so we
        // wire each Mermaid theme key to the canonical AGH token. Retunes
        // flow from `packages/ui/src/tokens.css` without touching this file.
        mermaid.initialize({
          startOnLoad: false,
          securityLevel: "strict",
          theme: "base",
          themeVariables: {
            background: "var(--color-rail)",
            primaryColor: "var(--color-canvas-soft)",
            primaryBorderColor: "var(--color-accent)",
            primaryTextColor: "var(--color-fg)",
            secondaryColor: "var(--color-elevated)",
            tertiaryColor: "var(--color-accent-ink)",
            lineColor: "var(--color-muted)",
            textColor: "var(--color-fg)",
            mainBkg: "var(--color-canvas-soft)",
            nodeBorder: "var(--color-accent)",
            clusterBkg: "var(--color-accent-ink)",
            clusterBorder: "var(--color-elevated)",
            edgeLabelBackground: "var(--color-accent-ink)",
            actorBkg: "var(--color-canvas-soft)",
            actorBorder: "var(--color-accent)",
            actorTextColor: "var(--color-fg)",
            noteBkgColor: "var(--color-canvas-soft)",
            noteBorderColor: "var(--color-elevated)",
            noteTextColor: "var(--color-muted)",
            fontFamily: "var(--font-sans)",
          },
        });
        return mermaid;
      })
      .catch(error => {
        mermaidLoader = null;
        throw error;
      });
  }

  return mermaidLoader;
}

export function Mermaid({ chart, caption }: { chart: string; caption?: string }) {
  const reactId = useId();
  const diagramId = `agh-mermaid-${reactId.replace(/[^a-zA-Z0-9_-]/g, "")}`;
  const containerRef = useRef<HTMLDivElement | null>(null);
  const [state, dispatch] = useReducer(
    (_state: { svg: string; error: string }, nextState: { svg?: string; error?: string }) => ({
      svg: nextState.svg ?? "",
      error: nextState.error ?? "",
    }),
    { svg: "", error: "" }
  );

  useEffect(() => {
    let active = true;

    dispatch({});

    void loadMermaid()
      .then(mermaid => {
        if (!active) return;
        return mermaid.render(diagramId, chart).then(rendered => {
          if (!active) return;
          dispatch({
            svg: rendered.svg.replace(
              "<svg ",
              '<svg aria-hidden="true" class="agh-mermaid-svg" data-theme="agh" '
            ),
          });
        });
      })
      .catch(err => {
        if (!active) return;
        dispatch({ error: err instanceof Error ? err.message : String(err) });
      });

    return () => {
      active = false;
    };
  }, [chart, diagramId]);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    container.replaceChildren();
    if (!state.svg) return;

    const parsed = new DOMParser().parseFromString(state.svg, "image/svg+xml");
    const parseError = parsed.querySelector("parsererror");
    if (parseError) {
      dispatch({ error: parseError.textContent ?? "Mermaid SVG parse failed." });
      return;
    }

    container.append(document.importNode(parsed.documentElement, true));
  }, [state.svg]);

  return (
    <figure className="not-prose my-6 overflow-x-auto rounded-lg border border-line bg-canvas-soft p-4">
      {state.svg ? (
        <div
          ref={containerRef}
          aria-label={caption ? `Diagram: ${caption}` : "Mermaid diagram"}
          className="agh-mermaid [&_svg]:h-auto [&_svg]:max-w-full"
          role="img"
        />
      ) : state.error ? (
        <div>
          <Eyebrow className="text-accent">Diagram source</Eyebrow>
          <p className="mt-2 text-sm leading-6 text-muted">
            Mermaid could not render this diagram in the current browser session.
          </p>
          <pre className="mt-4 overflow-x-auto rounded-md border border-line bg-rail p-3 text-xs leading-6 text-muted">
            <code>{chart}</code>
          </pre>
          <p className="mt-3 text-sm leading-6 text-subtle">{state.error}</p>
        </div>
      ) : (
        <p className="text-sm leading-6 text-muted">Rendering diagram…</p>
      )}

      {caption ? (
        <figcaption className="mt-3 text-sm leading-6 text-muted">{caption}</figcaption>
      ) : null}
    </figure>
  );
}
