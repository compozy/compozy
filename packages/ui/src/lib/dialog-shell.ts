/**
 * Host size contract for entity-editor modals.
 *
 * Every entity editor routes its `DialogContent` through {@link dialogShellClass}
 * so the host width is a token contract instead of an ad-hoc `max-w-*`.
 *
 * | Size | Width token | Height ceiling | Surfaces |
 * | --- | --- | --- | --- |
 * | `sm` | `--width-modal-sm` 560 | `--height-modal-tall` | start session · create/edit knowledge · create/edit channel · add vault secret |
 * | `md` | `--width-modal-md` 720 | `--height-modal-md` | task editor · add/edit MCP server · create/edit sandbox profile · edit bridge |
 * | `lg` | `--width-modal-lg` 880 | `--height-modal-tall` | create agent · create bridge |
 * | `xl` | `--width-modal-xl` 1180 | `--height-modal-xl` | job/trigger editor · add workspace (split body) |
 *
 * No `sheet` size is emitted: the provider surface stays on
 * `provider-detail-dialog.tsx`, so a 576px sheet host has no consumer.
 *
 * Height is a ceiling, never a floor, unless `fill` pins the host to its height
 * token — used by hosts that must not resize as their body changes (task
 * editor, automation editor). The body remains the only scroll owner.
 */
export type DialogShellSize = "sm" | "md" | "lg" | "xl";

export interface DialogShellOptions {
  /** Pin the host to its height token instead of sizing to content. */
  fill?: boolean;
}

const WIDTH_CLASS: Record<DialogShellSize, string> = {
  sm: "w-(--width-modal-sm) max-w-[calc(100vw-2rem)] sm:max-w-[min(var(--width-modal-sm),calc(100vw-2rem))]",
  md: "w-(--width-modal-md) max-w-[calc(100vw-2rem)] sm:max-w-[min(var(--width-modal-md),calc(100vw-2rem))]",
  lg: "w-(--width-modal-lg) max-w-[calc(100vw-2rem)] sm:max-w-[min(var(--width-modal-lg),calc(100vw-2rem))]",
  xl: "w-(--width-modal-xl) max-w-[calc(100vw-2rem)] sm:max-w-[min(var(--width-modal-xl),calc(100vw-2rem))]",
};

const MAX_HEIGHT_CLASS: Record<DialogShellSize, string> = {
  sm: "max-h-[min(var(--height-modal-tall),calc(100vh-2rem))]",
  md: "max-h-[min(var(--height-modal-md),calc(100vh-2rem))]",
  lg: "max-h-[min(var(--height-modal-tall),calc(100vh-2rem))]",
  xl: "max-h-[min(var(--height-modal-xl),calc(100vh-2rem))]",
};

const FILL_HEIGHT_CLASS: Record<DialogShellSize, string> = {
  sm: "h-(--height-modal-tall)",
  md: "h-(--height-modal-md)",
  lg: "h-(--height-modal-tall)",
  xl: "h-(--height-modal-xl)",
};

/** Grid column track that lets every shell row shrink below its content width. */
const SHELL_GRID_CLASS = "grid-cols-[minmax(0,1fr)]";

/**
 * At 760px and below a dialog becomes a full-width bottom surface
 * (`MODAL-STANDARD.md` § Hosts). The host drops its centering transform, spans
 * the viewport, and squares off its bottom corners against the screen edge.
 * `w-full` with no horizontal inset is what keeps the page free of sideways
 * scroll at 360px.
 */
const BOTTOM_SURFACE_CLASS =
  "max-[760px]:inset-x-0 max-[760px]:top-auto max-[760px]:bottom-0 max-[760px]:left-0 max-[760px]:w-full max-[760px]:max-w-full max-[760px]:translate-x-0 max-[760px]:translate-y-0 max-[760px]:rounded-b-none max-[760px]:max-h-[calc(100vh-2rem)] max-[760px]:h-auto";

/**
 * Comfort target for coarse pointers — `MODAL-STANDARD.md` § Hosts requires
 * interactive targets to reach 44px at 760px and below. Shared by every shell
 * control so the floor is defined once, from `--height-button-cta-lg`.
 */
export const DIALOG_TOUCH_TARGET_CLASS = "max-[760px]:min-h-(--height-button-cta-lg)";

/** Square variant of {@link DIALOG_TOUCH_TARGET_CLASS} for icon-only controls. */
export const DIALOG_TOUCH_TARGET_SQUARE_CLASS = "max-[760px]:size-(--height-button-cta-lg)";

/**
 * {@link DIALOG_TOUCH_TARGET_CLASS} projected onto segmented-control children.
 * Written as one literal so Tailwind's source scan can see it — composing this
 * selector at runtime would emit no CSS.
 */
export const DIALOG_TOUCH_TARGET_SEGMENTS_CLASS =
  "[&>[data-slot=pill-group-item]]:max-[760px]:min-h-(--height-button-cta-lg)";

/**
 * Token classes for an entity-editor `DialogContent` host.
 *
 * @example
 * <DialogContent unframed showCloseButton={false} className={dialogShellClass("md")}>
 */
export function dialogShellClass(size: DialogShellSize, options: DialogShellOptions = {}): string {
  const height = options.fill
    ? `${FILL_HEIGHT_CLASS[size]} ${MAX_HEIGHT_CLASS[size]}`
    : MAX_HEIGHT_CLASS[size];
  return `${SHELL_GRID_CLASS} ${WIDTH_CLASS[size]} ${height} ${BOTTOM_SURFACE_CLASS}`;
}
