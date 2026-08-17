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

import { cn } from "../lib/utils";

function Toaster({
  className,
  closeButton = true,
  icons,
  style,
  theme = "dark",
  toastOptions,
  ...props
}: ToasterProps) {
  return (
    <Sonner
      closeButton={closeButton}
      theme={theme}
      className={cn("toaster group", className)}
      icons={{
        success: <CircleCheckIcon className="size-4 text-success" />,
        info: <InfoIcon className="size-4 text-info" />,
        warning: <TriangleAlertIcon className="size-4 text-warning" />,
        error: <OctagonXIcon className="size-4 text-danger" />,
        loading: <Loader2Icon className="size-4 animate-spin text-muted" />,
        ...icons,
      }}
      style={
        {
          "--normal-bg": "var(--color-canvas-soft)",
          "--normal-text": "var(--color-fg)",
          "--normal-border": "var(--color-line-soft)",
          "--border-radius": "var(--radius-lg)",
          ...style,
        } as React.CSSProperties
      }
      toastOptions={{
        ...toastOptions,
        classNames: {
          ...toastOptions?.classNames,
          toast: cn(toastOptions?.classNames?.toast, "pointer-events-none"),
          actionButton: cn(toastOptions?.classNames?.actionButton, "pointer-events-auto"),
          cancelButton: cn(toastOptions?.classNames?.cancelButton, "pointer-events-auto"),
          closeButton: cn(toastOptions?.classNames?.closeButton, "pointer-events-auto"),
        },
      }}
      {...props}
    />
  );
}

export { Toaster, type ToasterProps };
