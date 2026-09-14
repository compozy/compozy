import {
  AbsoluteFill,
  Easing,
  interpolate,
  Sequence,
  useCurrentFrame,
  useVideoConfig,
} from "remotion";
import { color } from "@/brand/tokens";
import { LogoSting } from "@/components/logo-sting";
import { KineticCenterBuild } from "@/components/remocn/kinetic-center-build";
import { ReleaseBackdrop } from "@/components/release-backdrop";
import { VersionChip } from "@/components/version-chip";
import type { ReleaseAnnounceProps } from "./schema";

/** Seconds-based choreography remains consistent across output frame rates. */
export function ReleaseAnnounce({ version, features }: ReleaseAnnounceProps) {
  const frame = useCurrentFrame();
  const { width, height, fps, durationInFrames } = useVideoConfig();
  const portrait = width / height < 0.8;
  const unit = Math.min(width / 1920, height / 1080);
  const fontSize = portrait ? 76 : width < 1200 ? 64 : 104;
  return (
    <AbsoluteFill>
      <ReleaseBackdrop />
      <AbsoluteFill
        style={{
          alignItems: "center",
          justifyContent: "center",
          gap: 42 * unit,
          scale: interpolate(frame, [0, durationInFrames - 1], [1, 1.035]),
          translate: `0 ${interpolate(frame, [0, 1.25 * fps], [42 * unit, 0], {
            extrapolateRight: "clamp",
            easing: Easing.bezier(0.22, 1, 0.36, 1),
          })}px`,
        }}
      >
        <LogoSting height={portrait ? 106 : 122 * unit} />
        <Sequence from={Math.round(0.48 * fps)} layout="none" name="Release version">
          <VersionChip version={version} fontSize={portrait ? 30 : 32 * unit} />
        </Sequence>
        <div
          style={{
            position: "relative",
            width: "88%",
            height: features.length * fontSize * 1.25,
            marginTop: 12 * unit,
          }}
        >
          {features.map((feature, index) => (
            <div
              key={index}
              style={{
                position: "absolute",
                top: index * fontSize * 1.25,
                height: fontSize * 1.25,
                width: "100%",
              }}
            >
              <Sequence from={Math.round((0.82 + index * 0.3) * fps)} layout="none" name={feature}>
                <KineticCenterBuild
                  text={feature}
                  fontSize={fontSize}
                  maxWidth={width * 0.84}
                  fontWeight={500}
                  color={color.fgStrong}
                  speed={30 / fps}
                  entryOffset={70}
                />
              </Sequence>
            </div>
          ))}
        </div>
      </AbsoluteFill>
    </AbsoluteFill>
  );
}
