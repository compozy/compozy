import { cn } from "@agh/ui";

export interface BlogAvatarProps {
  initial: string;
  size?: "sm" | "md" | "lg";
  className?: string;
}

const sizeClass: Record<NonNullable<BlogAvatarProps["size"]>, string> = {
  sm: "size-7 text-xs",
  md: "size-9 text-sm",
  lg: "size-11 text-base",
};

export function BlogAvatar({ initial, size = "sm", className }: BlogAvatarProps) {
  return (
    <span
      aria-hidden
      className={cn(
        "inline-flex shrink-0 items-center justify-center rounded-full bg-elevated font-sans font-semibold text-fg",
        sizeClass[size],
        className
      )}
    >
      {initial}
    </span>
  );
}
