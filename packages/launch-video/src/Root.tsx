import { Composition } from "remotion";
import { ReleaseFilm } from "@/compositions/release-film";
import { releaseFilmSchema, filmDuration } from "@/compositions/release-film/schema";
import beta27 from "@/releases/beta27.json";
const filmDefaults = releaseFilmSchema.parse(beta27);

import { ReleaseAnnounce } from "@/compositions/release-announce";
import {
  DURATION_IN_FRAMES,
  FPS,
  releaseAnnounceDefaults,
} from "@/compositions/release-announce/props";
import { releaseAnnounceSchema } from "@/compositions/release-announce/schema";

const SHARED = {
  component: ReleaseAnnounce,
  schema: releaseAnnounceSchema,
  defaultProps: releaseAnnounceDefaults,
  durationInFrames: DURATION_IN_FRAMES,
  fps: FPS,
} as const;

export function RemotionRoot() {
  return (
    <>
      <Composition
        id="ReleaseFilm"
        component={ReleaseFilm}
        schema={releaseFilmSchema}
        defaultProps={filmDefaults}
        width={1920}
        height={1080}
        fps={60}
        durationInFrames={filmDuration(filmDefaults, 60)}
        calculateMetadata={({ props }) => ({
          durationInFrames: filmDuration(releaseFilmSchema.parse(props), 60),
          defaultOutName: `compozyos-${props.version}`,
        })}
      />
      <Composition id="ReleaseAnnounce" {...SHARED} width={1920} height={1080} />
      <Composition id="ReleaseAnnounceSquare" {...SHARED} width={1080} height={1080} />
      <Composition id="ReleaseAnnounceVertical" {...SHARED} width={1080} height={1920} />
    </>
  );
}
