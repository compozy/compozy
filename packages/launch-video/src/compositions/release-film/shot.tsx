import { Video } from "@remotion/media";
import {
  AbsoluteFill,
  Img,
  Easing,
  interpolate,
  staticFile,
  useCurrentFrame,
  useVideoConfig,
} from "remotion";
import { Cursor } from "@/components/remocn/cursor";
import { useCursorPath } from "@/components/remocn/use-cursor-path";
import type { Shot } from "./schema";

function Pointer({ shot, size }: { shot: Shot; size: number }) {
  const { fps } = useVideoConfig();
  // The registry's click pulse uses a 30fps clock; cues are authored in seconds.
  const path = useCursorPath(
    shot.cursor.map(p => ({ ...p, at: p.at * 30, duration: (p.duration ?? 0.5) * 30 })),
    { speed: 30 / fps }
  );
  const frame = useCurrentFrame();
  return (
    <div
      style={{ opacity: interpolate(frame, [0, 0.2 * fps], [0, 1], { extrapolateRight: "clamp" }) }}
    >
      <Cursor
        style={path}
        size={size}
        rippleColor="#e8572a"
        theme={{ foreground: "#ffffff", background: "#171615" }}
      />
    </div>
  );
}

/** Screenshot and cursor share coordinates, including every camera transform. */
function CatalogClick({ shot }: { shot: Shot }) {
  const frame = useCurrentFrame();
  const { durationInFrames } = useVideoConfig();
  return (
    <AbsoluteFill style={{ backgroundColor: "#111111", overflow: "hidden" }}>
      <div
        style={{
          position: "absolute",
          width: 1440,
          height: 900,
          left: 80,
          top: 60,
          transformOrigin: "0 0",
          scale: interpolate(frame, [0, durationInFrames - 1], [2.3, 2.4]),
          translate: `${interpolate(frame, [0, durationInFrames - 1], [0, -30])}px ${interpolate(frame, [0, durationInFrames - 1], [-40, -65])}px`,
        }}
      >
        <Img src={staticFile(shot.src)} style={{ width: 1440, height: 900 }} />
        <Pointer shot={shot} size={22} />
      </div>
    </AbsoluteFill>
  );
}

/** Keep the session, composer indicator, and inspector in one coordinate space. */
function ContextClick({ shot }: { shot: Shot }) {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const t = frame / fps;
  const ease = Easing.bezier(0.65, 0, 0.25, 1);
  const move = (times: number[], values: number[]) =>
    interpolate(t, times, values, {
      extrapolateLeft: "clamp",
      extrapolateRight: "clamp",
      easing: ease,
    });
  const clicks = shot.cursor.filter(p => p.click);
  const openAt = clicks[0]!.at;
  const expandAt = clicks[1]!.at;
  const opened = move([openAt, openAt + 0.4], [0, 1]);
  const expanded = move([expandAt, expandAt + 0.3], [0, 1]);
  const tooltip = move([1.45, 1.65, openAt - 0.12, openAt], [0, 1, 1, 0]);
  const cameraTimes = [0, 0.55, 1.55, 3.15, 4.3, 7];
  const scale = move(cameraTimes, [1.25, 1.27, 2.7, 2.7, 1.3, 1.33]);
  const x = move(cameraTimes, [60, 50, 173, 173, 24, -2]);
  const y = move(cameraTimes, [-15, -25, -1295, -1295, -35, -50]);
  const imageStyle = { position: "absolute" as const, width: 1440, height: 900, left: 0, top: 0 };
  return (
    <AbsoluteFill style={{ backgroundColor: "#111111", overflow: "hidden" }}>
      <div
        style={{
          position: "absolute",
          width: 1440,
          height: 900,
          transformOrigin: "0 0",
          transform: `translate(${x}px, ${y}px) scale(${scale})`,
        }}
      >
        <Img src={staticFile(shot.src)} style={imageStyle} />
        <Img src={staticFile(shot.hoverSrc!)} style={{ ...imageStyle, opacity: tooltip }} />
        <Img src={staticFile(shot.secondarySrc!)} style={{ ...imageStyle, opacity: opened }} />
        <Img src={staticFile(shot.expandedSrc!)} style={{ ...imageStyle, opacity: expanded }} />
        <Pointer shot={shot} size={20} />
      </div>
    </AbsoluteFill>
  );
}

export function ReleaseShot({ shot }: { shot: Shot }) {
  const { fps } = useVideoConfig();
  if (shot.kind === "catalog-click") return <CatalogClick shot={shot} />;
  if (shot.kind === "context-click") return <ContextClick shot={shot} />;
  return (
    <AbsoluteFill style={{ backgroundColor: "#111111" }}>
      <Video
        src={staticFile(shot.src)}
        trimBefore={Math.round(shot.sourceStart * fps)}
        trimAfter={Math.round((shot.sourceStart + shot.duration) * fps)}
        muted
        style={{ width: "100%", height: "100%" }}
        objectFit="cover"
      />
    </AbsoluteFill>
  );
}
