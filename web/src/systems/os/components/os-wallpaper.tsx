import { cn } from "@/lib/utils";

/**
 * The desktop wallpaper layer. `flat` (the default) is the plain desk surface
 * behind the panes; the others compose the tokenized gradient stack with the
 * dotted grid overlay — geometry lives in `:root` tokens (`--wallpaper-*`),
 * never as raw literals here.
 */
export type OsWallpaperKind = "flat" | "ember" | "mesh" | "carbon";

const WALLPAPER_BACKGROUND: Record<OsWallpaperKind, string> = {
  flat: "var(--wallpaper-flat)",
  ember: "var(--wallpaper-ember)",
  mesh: "var(--wallpaper-mesh)",
  carbon: "var(--wallpaper-carbon)",
};

export interface OsWallpaperProps extends React.ComponentProps<"div"> {
  /** Active wallpaper stack. */
  wallpaper?: OsWallpaperKind;
}

export function OsWallpaper({ wallpaper = "flat", className, style, ...props }: OsWallpaperProps) {
  // The flat desk carries no grid; the gradient wallpapers keep it.
  const grid = wallpaper !== "flat";
  return (
    <div
      data-slot="os-wallpaper"
      data-wallpaper={wallpaper}
      aria-hidden="true"
      className={cn("absolute inset-0", className)}
      style={{
        backgroundImage: grid
          ? `var(--wallpaper-grid), ${WALLPAPER_BACKGROUND[wallpaper]}`
          : WALLPAPER_BACKGROUND[wallpaper],
        backgroundSize: grid
          ? `var(--wallpaper-grid-size) var(--wallpaper-grid-size), auto, auto, auto`
          : undefined,
        ...style,
      }}
      {...props}
    />
  );
}
