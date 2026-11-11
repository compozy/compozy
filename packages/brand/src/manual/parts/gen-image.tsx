// A generated raster with provenance caption. Images live in public/brand/img/<area>/<id>.jpg.
import { cn } from "@compozy/ui/utils";

interface GenImageProps {
  src: string;
  alt: string;
  caption?: string;
  ratio?: string;
  className?: string;
  generated?: boolean;
}

export function GenImage({ src, alt, caption, ratio, className, generated = true }: GenImageProps) {
  return (
    <figure className={cn("m-0 min-w-0", className)}>
      <div
        className="overflow-hidden rounded-[14px] bg-(--b-card) shadow-[0_0_0_1px_var(--b-line)]"
        style={ratio ? { aspectRatio: ratio } : undefined}
      >
        <img
          src={src}
          alt={alt}
          loading="lazy"
          decoding="async"
          className="block h-full w-full object-cover"
        />
      </div>
      {(caption || generated) && (
        <figcaption className="mt-2.5 text-sm text-(--b-muted)">
          {caption}
          {generated && (
            <span className="ml-1 mono-label text-[11px] tracking-[0.08em]">
              {caption ? "· " : ""}generated reference
            </span>
          )}
        </figcaption>
      )}
    </figure>
  );
}
