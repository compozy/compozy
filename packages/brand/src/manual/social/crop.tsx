// An avatar template shown the way the network crops it (circle or rounded square) at several sizes.
import { cn } from "@compozy/ui/utils";

import { Frame } from "../kit-frame";

const SHAPE = { circle: "rounded-full", rounded: "rounded-[18%]", square: "rounded-[6%]" };
const FLAT = "overflow-hidden [&_figure>div]:rounded-none [&_figure>div]:shadow-none";

interface CropProps {
  id: string;
  shape: keyof typeof SHAPE;
  label: string;
}

export function Crop({ id, shape, label }: CropProps) {
  const r = SHAPE[shape];
  return (
    <figure className="m-0 min-w-0">
      <div className="band-paper flex items-end gap-3 rounded-[14px] p-5">
        {["w-[58%]", "w-[22%]", "w-[10%]"].map(w => (
          <div key={w} className={cn(w, FLAT, r)}>
            <Frame id={id} caption={false} />
          </div>
        ))}
      </div>
      <figcaption className="mt-3 flex flex-wrap items-center justify-between gap-2 mono-label text-[11.5px] tracking-[0.08em] text-(--b-muted)">
        <span>{label}</span>
        <span className="text-(--b-accent)">kit/{id}</span>
      </figcaption>
    </figure>
  );
}
