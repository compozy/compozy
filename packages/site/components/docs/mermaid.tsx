"use client";

import { useEffect, useId, useReducer, useRef } from "react";
import { DocsCode } from "@/components/docs/docs-prose";

let mermaidLoader: Promise<typeof import("mermaid").default> | null = null;

function loadMermaid() {
  if (!mermaidLoader) {
    mermaidLoader = import("mermaid")
      .then(({ default: mermaid }) => {
        // Mermaid emits SVG with `fill` / `stroke` attributes set to these
        // theme variable values. SVG attributes accept `var(--…)`, so we
        // wire each Mermaid theme key to the canonical CompozyOS token. Retunes
        // flow from `packages/ui/src/tokens.css` without touching this file.
        mermaid.initialize({
          startOnLoad: false,
          securityLevel: "strict",
          theme: "base",
          // The brand palette of the docs draft: diagrams sit on the dark surface, drawn in
          // hairlines with brand-blue edges.
          themeVariables: {
            background: "#0d0d0d",
            primaryColor: "#141414",
            primaryBorderColor: "rgb(255 255 255 / 0.2)",
            primaryTextColor: "#ececec",
            secondaryColor: "#181818",
            tertiaryColor: "#101010",
            lineColor: "#1f87fc",
            textColor: "#ececec",
            mainBkg: "#141414",
            nodeBorder: "rgb(255 255 255 / 0.2)",
            clusterBkg: "#101010",
            clusterBorder: "rgb(255 255 255 / 0.12)",
            edgeLabelBackground: "#0d0d0d",
            actorBkg: "#141414",
            actorBorder: "rgb(255 255 255 / 0.2)",
            actorTextColor: "#ececec",
            actorLineColor: "#1f87fc",
            signalColor: "#1f87fc",
            signalTextColor: "#ececec",
            noteBkgColor: "#181818",
            noteBorderColor: "rgb(255 255 255 / 0.12)",
            noteTextColor: "#989898",
            // Labels are measured before they are styled: a CSS variable does not resolve there,
            // so Mermaid gets the page's resolved family.
            fontFamily: getComputedStyle(document.body).fontFamily,
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
  const diagramId = `compozy-mermaid-${reactId.replace(/[^a-zA-Z0-9_-]/g, "")}`;
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
          dispatch({ svg: rendered.svg });
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

    const svg = document.importNode(parsed.documentElement, true);
    // Mermaid's SVG already carries a `class`; mark it through the DOM rather than the markup,
    // where a second `class` attribute makes the XML parser reject the whole diagram.
    svg.setAttribute("aria-hidden", "true");
    svg.setAttribute("data-theme", "compozy");
    svg.classList.add("compozy-mermaid-svg");
    container.append(svg);
  }, [state.svg]);

  return (
    <figure className="not-prose my-9">
      {state.svg ? (
        <div className="flex justify-center overflow-x-auto rounded-[16px] border border-site-light-080 bg-site-light-025 p-7">
          <div
            ref={containerRef}
            aria-label={caption ? `Diagram: ${caption}` : "Mermaid diagram"}
            className="compozy-mermaid flex w-full justify-center [&_svg]:h-auto [&_svg]:max-h-[520px] [&_svg]:max-w-full"
            role="img"
          />
        </div>
      ) : state.error ? (
        <div>
          <p className="m-0 text-[14px] text-muted-foreground">
            Mermaid could not render this diagram in the current browser session.
          </p>
          <DocsCode code={chart} language="mermaid" label="Diagram source" />
          <p className="m-0 text-[13px] text-subtle">{state.error}</p>
        </div>
      ) : (
        <div className="flex min-h-40 items-center justify-center rounded-[16px] border border-site-light-080 bg-site-light-025 text-[14px] text-muted-foreground">
          Rendering diagram…
        </div>
      )}

      {caption ? (
        <figcaption className="mt-3 text-center text-[13.5px] text-subtle">{caption}</figcaption>
      ) : null}
    </figure>
  );
}
