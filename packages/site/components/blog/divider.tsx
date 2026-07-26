import { cn } from "@agh/ui";

export interface BulletDividerProps {
  className?: string;
}

export function BulletDivider({ className }: BulletDividerProps) {
  return <span aria-hidden className={cn("inline-block h-3 w-px bg-line", className)} />;
}
