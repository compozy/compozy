// A product visual cropped and scaled so its text stays readable at the shown size.
// scale > 1 zooms into the focal point (ox, oy in %), so one window fills the box.
// flat=false asks for the built-in tilt; the product-shot frame (slice S3) owns the angle.
import { cn } from "@compozy/ui/utils";

import { Visual } from "../art/visual";
import type { VisualName } from "../art/visuals";

interface UiCropProps {
  name: VisualName;
  ratio?: string;
  scale?: number;
  ox?: number;
  oy?: number;
  flat?: boolean;
  theme?: "dark" | "light";
  className?: string;
}

export function UiCrop({
  name,
  ratio = "4/3",
  scale = 1,
  ox = 50,
  oy = 50,
  flat = true,
  theme = "dark",
  className,
}: UiCropProps) {
  return (
    <div
      data-tilt={flat ? "flat" : "built-in"}
      className={cn("relative overflow-hidden rounded-card border border-(--b-line)", className)}
      style={{ aspectRatio: ratio }}
    >
      <div
        className="absolute inset-0"
        style={{ transform: `scale(${scale})`, transformOrigin: `${ox}% ${oy}%` }}
      >
        <Visual name={name} x={0.5} y={0.5} theme={theme} inline ratio={ratio} />
      </div>
    </div>
  );
}
