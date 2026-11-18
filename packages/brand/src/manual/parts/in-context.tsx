// A row of generated mockups showing finished pieces in place.
import { cn } from "@compozy/ui/utils";

import { lab } from "../cls";
import { imageById, imgSrc } from "../images";
import { GenImage } from "./gen-image";

const COLS: Record<number, string> = { 1: "", 2: "md:grid-cols-2" };

export function InContext({
  ids,
  title = "In context",
  className,
}: {
  ids: string[];
  title?: string;
  className?: string;
}) {
  return (
    <div className={cn("mt-12", className)}>
      <p className={lab}>{title}</p>
      <div className={cn("mt-4 grid gap-4", COLS[ids.length] ?? "md:grid-cols-3")}>
        {ids.map(id => {
          const i = imageById(id);
          return (
            <GenImage key={id} src={imgSrc(i)} alt={i.alt} caption={i.caption} ratio={i.ratio} />
          );
        })}
      </div>
    </div>
  );
}
