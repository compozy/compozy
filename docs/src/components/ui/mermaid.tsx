"use client";

import { cn } from "@/lib/utils";
import mermaid from "mermaid";
import { useTheme } from "next-themes";
import { useCallback, useEffect, useRef, useState } from "react";
import { Controlled as ControlledZoom } from "react-medium-image-zoom";
import "react-medium-image-zoom/dist/styles.css";

interface MermaidProps {
  chart: string;
  config?: any;
  className?: string;
}

export function Mermaid({ chart, config = {}, className }: MermaidProps) {
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
      fontFamily: "Geist, ui-sans-serif, system-ui, sans-serif",
      themeVariables: {
        fontFamily: "Geist, ui-sans-serif, system-ui, sans-serif",
        // primaryTextColor: resolvedTheme === "dark" ? "#C1E623" : "#3d3929",
        // primaryColor: resolvedTheme === "dark" ? "#1f1f1f" : "#e9e6dc",
        // primaryBorderColor: resolvedTheme === "dark" ? "#363736" : "#dad9d4",
        // lineColor: resolvedTheme === "dark" ? "#52514a" : "#b4b2a7",
        // secondaryColor: resolvedTheme === "dark" ? "#242424" : "#e9e6dc",
        // tertiaryColor: resolvedTheme === "dark" ? "#313131" : "#ede9de",
        // background: resolvedTheme === "dark" ? "#161716" : "#faf9f5",
        // mainBkg: resolvedTheme === "dark" ? "#1f1f1f" : "#e9e6dc",
        // secondBkg: resolvedTheme === "dark" ? "#242424" : "#ede9de",
        // tertiaryBkg: resolvedTheme === "dark" ? "#313131" : "#e9e6dc",
        // textColor: resolvedTheme === "dark" ? "#dce4e5" : "#3d3929",
        ...config.themeVariables,
      },
      flowchart: {
        nodeSpacing: 50,
        rankSpacing: 50,
        curve: "basis",
        padding: 15,
        htmlLabels: true,
        defaultRenderer: "dagre-d3",
        useMaxWidth: true,
      },
      sequence: {
        fontFamily: "Geist, ui-sans-serif, system-ui, sans-serif",
        fontSize: 14,
        messageFontSize: 14,
        noteFontSize: 13,
        actorFontSize: 14,
        actorFontFamily: "Geist, ui-sans-serif, system-ui, sans-serif",
        noteFontFamily: "Geist, ui-sans-serif, system-ui, sans-serif",
        messageFontFamily: "Geist, ui-sans-serif, system-ui, sans-serif",
      },
      gantt: {
        fontFamily: "Geist, ui-sans-serif, system-ui, sans-serif",
        fontSize: 14,
        sectionFontSize: 14,
      },
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
    <div className={cn("mermaid-container w-full", className)}>
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
