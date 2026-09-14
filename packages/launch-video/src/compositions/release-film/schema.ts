import { z } from "zod";
import { releaseAnnounceSchema } from "../release-announce/schema";

const point = z.object({
  at: z.number().min(0),
  x: z.number(),
  y: z.number(),
  duration: z.number().min(0).optional(),
  click: z.boolean().optional(),
});
const shot = z.object({
  name: z.string().min(1),
  kind: z.enum(["video", "catalog-click", "context-click"]),
  src: z.string().min(1),
  secondarySrc: z.string().optional(),
  hoverSrc: z.string().optional(),
  expandedSrc: z.string().optional(),
  sourceStart: z.number().min(0),
  duration: z.number().min(0.5),
  transition: z.enum(["push", "left", "right", "up", "fade"]),
  overlap: z.number().min(0.1).max(0.8),
  cursor: z.array(point),
});
export const releaseFilmSchema = releaseAnnounceSchema
  .extend({
    introDuration: z.number().min(2).max(6),
    shots: z.array(shot).min(1),
  })
  .superRefine((value, ctx) => {
    for (const [index, current] of value.shots.entries()) {
      const next = value.shots[index + 1];
      if (current.overlap >= current.duration || (next && current.overlap >= next.duration)) {
        ctx.addIssue({
          code: "custom",
          path: ["shots", index, "overlap"],
          message: "Overlap must be shorter than both scenes.",
        });
      }
      if (
        current.cursor.some(
          (p, i) => p.at >= current.duration || (i > 0 && p.at < current.cursor[i - 1]!.at)
        )
      ) {
        ctx.addIssue({
          code: "custom",
          path: ["shots", index, "cursor"],
          message: "Cursor arrivals must be ordered and inside the scene.",
        });
      }
      if (
        current.kind === "context-click" &&
        (!current.secondarySrc ||
          !current.hoverSrc ||
          !current.expandedSrc ||
          current.cursor.filter(p => p.click).length !== 2)
      ) {
        ctx.addIssue({
          code: "custom",
          path: ["shots", index],
          message:
            "Session context needs closed, hovered, open and expanded captures, plus two click cues.",
        });
      }
    }
  });
export type ReleaseFilmProps = z.infer<typeof releaseFilmSchema>;
export type Shot = ReleaseFilmProps["shots"][number];
export function timeline(props: ReleaseFilmProps, fps: number) {
  let start = Math.round(props.introDuration * fps) - Math.round(0.45 * fps);
  return props.shots.map((shot, index) => {
    const duration = Math.round(shot.duration * fps);
    const overlap = index < props.shots.length - 1 ? Math.round(shot.overlap * fps) : 0;
    const entry = { start, duration, overlap, name: shot.name };
    start += duration - overlap;
    return entry;
  });
}
export function filmDuration(props: ReleaseFilmProps, fps: number) {
  const last = timeline(props, fps).at(-1)!;
  return last.start + last.duration;
}
