// Mermaid owns mutable global configuration. Serialize configuration and rendering together so
// simultaneous diagrams cannot borrow another diagram's theme or security configuration.
let pending: Promise<unknown> = Promise.resolve();

/** Serializes strict, theme-aware rendering into an isolated SVG image with intrinsic dimensions. */
export function renderMermaidImage(code: string, theme: "light" | "dark"): Promise<string> {
  const result = pending.then(async () => {
    const { default: mermaid } = await import("mermaid");
    mermaid.initialize({
      startOnLoad: false,
      securityLevel: "strict",
      htmlLabels: false,
      flowchart: { htmlLabels: false },
      theme: theme === "dark" ? "dark" : "default",
      suppressErrorRendering: true,
      // Source directives/frontmatter must not relax the renderer's security policy.
      secure: [
        "securityLevel",
        "theme",
        "themeVariables",
        "startOnLoad",
        "htmlLabels",
        "flowchart",
        "secure",
        "maxTextSize",
        "maxEdges",
        "suppressErrorRendering",
      ],
    });
    const container = document.createElement("div");
    container.style.position = "absolute";
    container.style.visibility = "hidden";
    document.body.append(container);
    try {
      const { svg } = await mermaid.render(`mermaid-${crypto.randomUUID()}`, code, container);
      const svgDocument = new DOMParser().parseFromString(svg, "image/svg+xml");
      const root = svgDocument.documentElement;
      const viewBox = root
        .getAttribute("viewBox")
        ?.trim()
        .split(/[\s,]+/)
        .map(Number);
      // Mermaid uses percentage widths. Give image documents intrinsic dimensions so CSS
      // can shrink a diagram without stretching a small diagram to fill the transcript.
      if (
        viewBox?.length === 4 &&
        viewBox.every(Number.isFinite) &&
        viewBox[2]! > 0 &&
        viewBox[3]! > 0
      ) {
        root.setAttribute("width", String(viewBox[2]));
        root.setAttribute("height", String(viewBox[3]));
      }
      // SVG image documents disable scripts, interaction, and external resource loading. Never
      // insert generated SVG as Markdown HTML or relax the Markdown element allowlist.
      return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(new XMLSerializer().serializeToString(root))}`;
    } finally {
      container.remove();
    }
  });
  pending = result.catch(() => undefined);
  return result;
}
