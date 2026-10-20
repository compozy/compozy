import mermaid from "mermaid";

// Mermaid owns mutable global configuration. Serialize configuration and rendering together so
// simultaneous diagrams cannot borrow another diagram's theme or security configuration.
let pending: Promise<unknown> = Promise.resolve();

export function renderMermaidImage(code: string, theme: "light" | "dark"): Promise<string> {
  const result = pending.then(async () => {
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
      // SVG image documents disable scripts, interaction, and external resource loading. Never
      // insert generated SVG as Markdown HTML or relax the Markdown element allowlist.
      return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;
    } finally {
      container.remove();
    }
  });
  pending = result.catch(() => undefined);
  return result;
}
