"use client";

import * as React from "react";
import { Toaster as Sonner, type ToasterProps } from "sonner";
import {
  CircleCheckIcon,
  InfoIcon,
  Loader2Icon,
  OctagonXIcon,
  TriangleAlertIcon,
} from "lucide-react";

function Toaster({ closeButton = true, theme = "dark", ...props }: ToasterProps) {
  return (
    <Sonner
      closeButton={closeButton}
      theme={theme}
      className="toaster group"
      icons={{
        success: <CircleCheckIcon className="size-4 text-success" />,
        info: <InfoIcon className="size-4 text-info" />,
        warning: <TriangleAlertIcon className="size-4 text-warning" />,
        error: <OctagonXIcon className="size-4 text-danger" />,
        loading: <Loader2Icon className="size-4 animate-spin text-muted" />,
      }}
      style={
        {
          "--normal-bg": "var(--color-canvas-soft)",
          "--normal-text": "var(--color-fg)",
          "--normal-border": "var(--color-line-soft)",
          "--border-radius": "var(--radius-lg)",
        } as React.CSSProperties
      }
      {...props}
    />
  );
}

export { Toaster, type ToasterProps };
