/** Layout constants and the capture-camera geometry shared by the site drafts. */
export const CONTAINER = "mx-auto w-full max-w-[1280px] px-6 md:px-10";
export const HAIRLINE = "border-[color:var(--light-080)]";
/** Pinned stages fade into the page instead of starting at a hard edge. */
export const STAGE_FADE =
  "[mask-image:linear-gradient(180deg,transparent,black_20%,black_80%,transparent),linear-gradient(90deg,transparent,black_14%,black_86%,transparent)] [mask-composite:intersect]";

/** A region of a capture, as fractions of its width and height. The frame shares the capture's aspect. */
export interface Focus {
  x: number;
  y: number;
  w: number;
}

export const WHOLE: Focus = { x: 0, y: 0, w: 1 };

export function focusToTransform(focus: Focus) {
  const scale = 1 / focus.w;
  return { scale, x: `${-focus.x * scale * 100}%`, y: `${-focus.y * scale * 100}%` };
}
