/**
 * The single built-in attention chime.
 *
 * One sound per delivery batch, never one per toast — a fleet finishing at once
 * must not become a chorus. Playback failure is silent by contract: browser
 * autoplay policy and a missing output device are both ordinary conditions, not
 * errors the operator can act on, and the visual delivery already happened.
 */

const ATTENTION_SOUND_URL = "/sounds/attention.wav";

export interface AttentionSoundPlayer {
  play: () => void;
}

export function createAttentionSoundPlayer(
  createAudio: (src: string) => HTMLAudioElement = src => new Audio(src)
): AttentionSoundPlayer {
  let element: HTMLAudioElement | null = null;
  return {
    play: () => {
      try {
        element ??= createAudio(ATTENTION_SOUND_URL);
        element.currentTime = 0;
        // `play()` rejects under autoplay policy; swallowing it is the whole
        // point — the operator never sees an error for a sound they did not ask
        // to hear, and the toast is already on screen.
        void element.play().catch(() => undefined);
      } catch {
        // A browser without audio support gets visual delivery only.
      }
    },
  };
}

export { ATTENTION_SOUND_URL };
