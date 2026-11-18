/** A post's cover: brand art set behind the card or the article opening. */
import { cn } from "@compozy/ui";
import Image from "next/image";

import type { BlogEntryCover } from "@/lib/blog-index";

export function CoverArt({
  cover,
  sizes,
  priority = false,
  className,
}: {
  cover: BlogEntryCover;
  sizes: string;
  priority?: boolean;
  className?: string;
}) {
  return (
    <span className={cn("absolute inset-0 block overflow-hidden", className)}>
      <Image
        src={cover.src}
        alt=""
        fill
        sizes={sizes}
        priority={priority}
        draggable={false}
        placeholder={cover.blurDataURL ? "blur" : "empty"}
        blurDataURL={cover.blurDataURL}
        className="object-cover transition-transform duration-[1400ms] ease-out group-hover/post:scale-[1.06]"
      />
    </span>
  );
}
