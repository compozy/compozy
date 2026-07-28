import { cn } from "@/lib/utils";

/**
 * The desktop wallpaper layer. Composes the tokenized gradient stack for the
 * active wallpaper with the dotted grid overlay — gradient geometry lives in
 * `:root` tokens (`--wallpaper-*`), never as raw literals here.
 */
export type OsWallpaperKind = "ember" | "mesh" | "carbon";

const WALLPAPER_BACKGROUND: Record<OsWallpaperKind, string> = {
  ember: "var(--wallpaper-ember)",
  mesh: "var(--wallpaper-mesh)",
  carbon: "var(--wallpaper-carbon)",
};

export interface OsWallpaperProps extends React.ComponentProps<"div"> {
  /** Active wallpaper stack. */
  wallpaper?: OsWallpaperKind;
}

export function OsWallpaper({ wallpaper = "ember", className, style, ...props }: OsWallpaperProps) {
  return (
    <div
      data-slot="os-wallpaper"
      data-wallpaper={wallpaper}
      aria-hidden="true"
      className={cn("absolute inset-0", className)}
      style={{
        backgroundImage: `var(--wallpaper-grid), ${WALLPAPER_BACKGROUND[wallpaper]}`,
        backgroundSize: `var(--wallpaper-grid-size) var(--wallpaper-grid-size), auto, auto, auto`,
        ...style,
      }}
      {...props}
    />
  );
}
