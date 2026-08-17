// Suite: attention sound adapter
// Invariant: the chime plays from the one bundled asset, restarts rather than
// overlapping, and swallows every playback failure so a browser that refuses
// audio never surfaces an error the operator cannot act on.
// Owning layer: unit (systems/os/lib) — the audio I/O boundary.
import { describe, expect, it, vi } from "vitest";

import { ATTENTION_SOUND_URL, createAttentionSoundPlayer } from "../attention-sound";

function fakeAudio(play: () => Promise<void>) {
  const element = { currentTime: 3, play: vi.fn(play) };
  return element as unknown as HTMLAudioElement & { play: ReturnType<typeof vi.fn> };
}

describe("attention sound (UT-069)", () => {
  it("Should play the one bundled asset and rewind before each play", () => {
    const element = fakeAudio(() => Promise.resolve());
    const createAudio = vi.fn(() => element);
    const player = createAttentionSoundPlayer(createAudio);

    player.play();
    player.play();

    expect(createAudio).toHaveBeenCalledExactlyOnceWith(ATTENTION_SOUND_URL);
    expect(element.currentTime).toBe(0);
    expect(element.play).toHaveBeenCalledTimes(2);
  });

  it("Should swallow an autoplay rejection without surfacing an error", async () => {
    // Browsers reject `play()` until the operator has interacted with the page.
    // Visual delivery already happened; there is nothing to report and nothing
    // to act on.
    const element = fakeAudio(() => Promise.reject(new Error("NotAllowedError")));
    const player = createAttentionSoundPlayer(() => element);

    expect(() => player.play()).not.toThrow();
    await expect(Promise.resolve()).resolves.toBeUndefined();
  });

  it("Should stay silent when the environment cannot construct audio at all", () => {
    const player = createAttentionSoundPlayer(() => {
      throw new Error("no audio support");
    });

    expect(() => player.play()).not.toThrow();
  });
});
