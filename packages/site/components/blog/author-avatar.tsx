import { cn } from "@compozy/ui";

import type { PostAuthor } from "@/lib/blog";

/** The author's GitHub avatar, or their initial when no profile is known. */
export function AuthorAvatar({ author, className }: { author: PostAuthor; className: string }) {
  if (!author.avatar) {
    return (
      <span
        aria-hidden
        className={cn(
          "inline-flex shrink-0 items-center justify-center rounded-full bg-site-light-100 text-fg",
          className
        )}
      >
        {author.name.charAt(0).toUpperCase()}
      </span>
    );
  }
  // eslint-disable-next-line @next/next/no-img-element -- a remote GitHub avatar; next/image would need a remote pattern for one 96px image.
  return <img src={author.avatar} alt="" className={cn("shrink-0 rounded-full", className)} />;
}
