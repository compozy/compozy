"use client";

import * as React from "react";
import { encode } from "uqr";

import { cn } from "../../lib/utils";

export type QrCodeSize = "sm" | "default" | "lg";

export interface QrCodeProps extends Omit<React.ComponentProps<"svg">, "children"> {
  /** Payload encoded into the matrix. Encoded byte-for-byte. */
  value: string;
  /**
   * Accessible name. A QR matrix is opaque to assistive technology, so the
   * label must describe what scanning it does — never the raw payload.
   */
  label: string;
  /** Rendered edge length: `sm` 128 px, `default` 176 px, `lg` 224 px. */
  size?: QrCodeSize;
  /** Quiet-zone modules around the matrix. The spec minimum is 4. */
  quietZone?: number;
}

const SIZE_CLASSNAME: Record<QrCodeSize, string> = {
  sm: "size-32",
  default: "size-44",
  lg: "size-56",
};

/**
 * Renders `value` as a scannable QR matrix.
 *
 * The plate is painted `--color-viz-cell` and the modules `--color-accent-ink`
 * rather than inheriting the warm-dark ramp: phone scanners expect dark modules
 * on a light quiet zone, and an inverted code on `--color-canvas` fails to
 * decode on a large share of devices. Scannability is a functional contract, so
 * these two tokens are fixed regardless of surrounding surface.
 *
 * Every surface that shows a QR code must also offer the same payload as
 * copyable text: cameras are optional, the artifact is not.
 */
function QrCode({
  value,
  label,
  size = "default",
  quietZone = 4,
  className,
  ...props
}: QrCodeProps) {
  const matrix = encode(value, { border: quietZone, ecc: "M" });
  const path = buildModulePath(matrix.data);

  return (
    <svg
      data-slot="qr-code"
      data-size={size}
      role="img"
      aria-label={label}
      viewBox={`0 0 ${matrix.size} ${matrix.size}`}
      shapeRendering="crispEdges"
      className={cn("shrink-0 rounded-sm", SIZE_CLASSNAME[size], className)}
      {...props}
    >
      <rect width={matrix.size} height={matrix.size} className="fill-viz-cell" />
      <path d={path} className="fill-accent-ink" />
    </svg>
  );
}

/**
 * One `<path>` for the whole matrix: a module per `<rect>` costs thousands of
 * DOM nodes at version 10+, which is what makes naive QR components janky.
 * Horizontal runs are merged so a row of dark modules emits a single segment.
 */
function buildModulePath(modules: readonly (readonly boolean[])[]): string {
  const segments: string[] = [];
  for (let row = 0; row < modules.length; row++) {
    const cells = modules[row];
    let runStart = -1;
    for (let column = 0; column <= cells.length; column++) {
      const dark = column < cells.length && cells[column];
      if (dark && runStart === -1) {
        runStart = column;
        continue;
      }
      if (!dark && runStart !== -1) {
        segments.push(`M${runStart} ${row}h${column - runStart}v1h-${column - runStart}z`);
        runStart = -1;
      }
    }
  }
  return segments.join("");
}

export { QrCode };
