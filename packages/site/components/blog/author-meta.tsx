import { Eyebrow, cn } from "@compozy/ui";
import { BlogAvatar } from "./avatar";

export interface AuthorMetaProps {
  handle: string;
  href?: string;
  initial: string;
  role?: string;
  size?: "sm" | "md" | "lg";
  layout?: "row" | "stacked";
  className?: string;
}

export function AuthorMeta({
  handle,
  href,
  initial,
  role,
  size = "sm",
  layout = "row",
  className,
}: AuthorMetaProps) {
  const name = href ? (
    <a
      href={href}
      rel="author"
      className="underline decoration-line underline-offset-4 hover:decoration-current"
    >
      {handle}
    </a>
  ) : (
    handle
  );

  if (layout === "stacked") {
    return (
      <div className={cn("flex items-center gap-3", className)}>
        <BlogAvatar initial={initial} size={size} />
        <div>
          <p className="font-sans text-sm font-medium text-fg">{name}</p>
          {role && <Eyebrow className="text-muted">{role}</Eyebrow>}
        </div>
      </div>
    );
  }

  return (
    <div className={cn("inline-flex items-center gap-2.5", className)}>
      <BlogAvatar initial={initial} size={size} />
      <Eyebrow className="text-muted">{name}</Eyebrow>
    </div>
  );
}
