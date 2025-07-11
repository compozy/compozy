"use client";

import mermaid from "mermaid";
import { useTheme } from "next-themes";
import { useCallback, useEffect, useRef, useState } from "react";
import { Controlled as ControlledZoom } from "react-medium-image-zoom";
import "react-medium-image-zoom/dist/styles.css";

interface MermaidProps {
  chart: string;
  config?: any;
}

export function Mermaid({ chart, config = {} }: MermaidProps) {
  const { resolvedTheme } = useTheme();
  const [imageSrc, setImageSrc] = useState<string>("");
  const [error, setError] = useState<string>("");
  const [loading, setLoading] = useState(true);
  const [isZoomed, setIsZoomed] = useState(false);
  const idRef = useRef(`mermaid-${Math.random().toString(36).substr(2, 9)}`);

  const handleZoomChange = useCallback((shouldZoom: boolean) => {
    setIsZoomed(shouldZoom);
  }, []);

  // Initialize Mermaid with theme
  useEffect(() => {
    mermaid.initialize({
      startOnLoad: false,
      theme: resolvedTheme === "dark" ? "dark" : "default",
      securityLevel: "loose",
      fontFamily: "var(--font-geist-sans), ui-sans-serif, system-ui, sans-serif",
      ...config,
    });
  }, [resolvedTheme, config]);

  // Render diagram
  useEffect(() => {
    if (!chart) return;

    const renderDiagram = async () => {
      try {
        setLoading(true);
        setError("");

        // Render new diagram
        const { svg } = await mermaid.render(idRef.current, chart);
        // Convert SVG string to base64 data URL for use as img src
        const base64Svg = btoa(unescape(encodeURIComponent(svg)));
        setImageSrc(`data:image/svg+xml;base64,${base64Svg}`);
        setLoading(false);
      } catch (e: any) {
        console.error("Mermaid rendering error:", e);
        setError(e?.message || "Failed to render diagram");
        setLoading(false);
      }
    };

    renderDiagram();
  }, [chart, resolvedTheme]);

  if (error) {
    return (
      <div className="rounded-md bg-destructive/10 p-4 text-destructive">
        <p className="text-sm font-medium">Diagram Error</p>
        <p className="text-xs mt-1">{error}</p>
        <details className="mt-2">
          <summary className="text-xs cursor-pointer">View source</summary>
          <pre className="mt-2 text-xs overflow-x-auto">{chart}</pre>
        </details>
      </div>
    );
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center p-8 text-muted-foreground">
        <div className="animate-pulse">Loading diagram...</div>
      </div>
    );
  }

  return (
    <div className="mermaid-container w-full">
      <ControlledZoom
        isZoomed={isZoomed}
        onZoomChange={handleZoomChange}
        wrapElement="div"
        zoomMargin={50}
      >
        <img
          src={imageSrc}
          alt="Mermaid diagram - Click to zoom"
          style={{
            width: "fit-content",
            margin: "0 auto",
            display: "inline-block",
            cursor: isZoomed ? "zoom-out" : "zoom-in",
          }}
        />
      </ControlledZoom>
    </div>
  );
}
