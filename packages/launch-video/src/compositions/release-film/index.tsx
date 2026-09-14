import { AbsoluteFill, useVideoConfig } from "remotion";
import { TransitionSeries, linearTiming } from "@remotion/transitions";
import { fade } from "@remotion/transitions/fade";
import { whipPan } from "@/components/remocn/whip-pan";
import { pushThrough } from "@/components/remocn/push-through";
import { ReleaseAnnounce } from "../release-announce";
import { ReleaseShot } from "./shot";
import type { ReleaseFilmProps, Shot } from "./schema";

function transitionElement(shot: Shot, index: number, fps: number) {
  const timing = linearTiming({ durationInFrames: Math.round(shot.overlap * fps) });
  const key = `${index}-transition`;
  if (shot.transition === "push")
    return (
      <TransitionSeries.Transition
        key={key}
        timing={timing}
        presentation={pushThrough({ zoom: 1.65, blur: 9 })}
      />
    );
  if (shot.transition === "fade")
    return <TransitionSeries.Transition key={key} timing={timing} presentation={fade()} />;
  return (
    <TransitionSeries.Transition
      key={key}
      timing={timing}
      presentation={whipPan({ direction: shot.transition, blur: 15 })}
    />
  );
}
export function ReleaseFilm(props: ReleaseFilmProps) {
  const { fps } = useVideoConfig();
  return (
    <AbsoluteFill style={{ backgroundColor: "#111111" }}>
      <TransitionSeries>
        <TransitionSeries.Sequence
          durationInFrames={Math.round(props.introDuration * fps)}
          name="Release opening"
        >
          <ReleaseAnnounce version={props.version} features={props.features} />
        </TransitionSeries.Sequence>
        <TransitionSeries.Transition
          presentation={pushThrough({ zoom: 1.8, blur: 10 })}
          timing={linearTiming({ durationInFrames: Math.round(0.45 * fps) })}
        />
        {props.shots.flatMap((shot, index) => [
          <TransitionSeries.Sequence
            key={`${index}-shot`}
            durationInFrames={Math.round(shot.duration * fps)}
            name={shot.name}
          >
            <ReleaseShot shot={shot} />
          </TransitionSeries.Sequence>,
          ...(index < props.shots.length - 1 ? [transitionElement(shot, index, fps)] : []),
        ])}
      </TransitionSeries>
    </AbsoluteFill>
  );
}
