import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it } from "vitest";

import { destroyTerminalInstances } from "@compozy/ui";

import { AsciicastPlayback, parseAsciicast } from "../../lib/asciicast";
import { RECORDING_FIXTURE } from "../../mocks/terminal-fixtures";
import { TerminalRecordingPlayer } from "../terminal-recording-player";
import { stubEngineLoader } from "./terminal-window-harness";

/**
 * Canonical suite for recording replay (UT-085).
 *
 * Invariant: a fixture artifact replays with its recorded timing honoured — the
 * point of a recording is what the screen looked like over time, so a flat dump
 * of the file answers a different question.
 */

/** A clock the test drives, so timing is asserted rather than waited on. */
function createManualClock() {
  const pending: Array<{ at: number; run: () => void }> = [];
  let now = 0;
  return {
    now: () => now,
    schedule: (run: () => void, delayMs: number) => {
      const entry = { at: now + delayMs, run };
      pending.push(entry);
      return () => {
        const index = pending.indexOf(entry);
        if (index >= 0) pending.splice(index, 1);
      };
    },
    /**
     * Advances time, firing everything due in due order.
     *
     * The clock moves *to* each entry's own time before running it, the way
     * real time does: a callback that schedules the next gap must measure it
     * from when it actually fired, not from the end of the jump.
     */
    advanceTo: (target: number) => {
      for (;;) {
        const next = pending
          .filter(entry => entry.at <= target)
          .sort((left, right) => left.at - right.at)[0];
        if (!next) break;
        pending.splice(pending.indexOf(next), 1);
        now = next.at;
        next.run();
      }
      now = target;
    },
  };
}

afterEach(() => {
  destroyTerminalInstances(() => true);
});

describe("parseAsciicast", () => {
  it("Should read the header and every output frame with its offset", () => {
    const cast = parseAsciicast(RECORDING_FIXTURE);

    expect(cast.header).toEqual({ version: 2, width: 96, height: 28, title: "make gate" });
    expect(cast.frames.map(frame => frame.atMs)).toEqual([
      0, 400, 2100, 38_600, 44_900, 50_200, 131_400, 149_000,
    ]);
    expect(cast.durationMs).toBe(149_000);
  });

  it("Should refuse a format it cannot replay rather than guess at it", () => {
    expect(() => parseAsciicast(JSON.stringify({ version: 1 }))).toThrow(
      /Unsupported recording format/
    );
    expect(() => parseAsciicast("")).toThrow(/empty/);
  });

  it("Should drop anything that is not output rather than paint it as screen", () => {
    const withInput = [
      JSON.stringify({ version: 2, width: 96, height: 28 }),
      JSON.stringify([0.1, "i", "secret keystroke"]),
      JSON.stringify([0.2, "o", "visible output"]),
    ].join("\n");

    const cast = parseAsciicast(withInput);

    expect(cast.frames).toEqual([{ atMs: 200, data: "visible output" }]);
  });
});

describe("AsciicastPlayback", () => {
  it("Should release each frame at its recorded time, not all at once", () => {
    const clock = createManualClock();
    const written: string[] = [];
    const progress: number[] = [];
    const playback = new AsciicastPlayback({
      cast: parseAsciicast(RECORDING_FIXTURE),
      sink: { write: data => written.push(data), reset: () => written.push("<reset>") },
      onProgress: position => progress.push(position),
      schedule: clock.schedule,
    });

    playback.play();
    expect(written).toEqual([]);

    clock.advanceTo(0);
    expect(written).toEqual(["$ make gate\r\n"]);

    clock.advanceTo(399);
    expect(written).toHaveLength(1);

    clock.advanceTo(400);
    expect(written).toHaveLength(2);

    clock.advanceTo(2100);
    expect(written).toHaveLength(3);

    // A minute of silence stays a minute of silence: nothing is released early.
    clock.advanceTo(38_599);
    expect(written).toHaveLength(3);

    clock.advanceTo(149_000);
    expect(written).toHaveLength(8);
    expect(written.at(-1)).toBe("gate: PASS\r\n");
    expect(progress).toEqual([0, 400, 2100, 38_600, 44_900, 50_200, 131_400, 149_000]);
  });

  it("Should stop releasing frames while paused", () => {
    const clock = createManualClock();
    const written: string[] = [];
    const playback = new AsciicastPlayback({
      cast: parseAsciicast(RECORDING_FIXTURE),
      sink: { write: data => written.push(data), reset: () => undefined },
      onProgress: () => undefined,
      schedule: clock.schedule,
    });

    playback.play();
    clock.advanceTo(400);
    expect(written).toHaveLength(2);

    playback.pause();
    clock.advanceTo(149_000);

    expect(written).toHaveLength(2);
  });

  it("Should rebuild the screen from the start when seeking", () => {
    const clock = createManualClock();
    const written: string[] = [];
    const playback = new AsciicastPlayback({
      cast: parseAsciicast(RECORDING_FIXTURE),
      sink: { write: data => written.push(data), reset: () => written.push("<reset>") },
      onProgress: () => undefined,
      schedule: clock.schedule,
    });

    playback.seek(2100);

    // The screen at 2.1s is everything up to 2.1s, replayed from a clean slate —
    // not whatever the emulator happened to be holding.
    expect(written).toEqual([
      "<reset>",
      "$ make gate\r\n",
      "gate: classifying diff vs merge-base…\r\n",
      "gate: go lane → go-lint + go test -race (scoped)\r\n",
    ]);
  });

  it("Should report the end of the capture once", () => {
    const clock = createManualClock();
    let ended = 0;
    const playback = new AsciicastPlayback({
      cast: parseAsciicast(RECORDING_FIXTURE),
      sink: { write: () => undefined, reset: () => undefined },
      onProgress: () => undefined,
      onEnded: () => {
        ended += 1;
      },
      schedule: clock.schedule,
    });

    playback.play();
    clock.advanceTo(200_000);

    expect(ended).toBe(1);
    expect(playback.isPlaying).toBe(false);
  });
});

describe("TerminalRecordingPlayer", () => {
  it("Should show the capture length and start paused", async () => {
    const clock = createManualClock();
    render(
      <TerminalRecordingPlayer
        engineLoader={stubEngineLoader}
        recordingId="rec-9f21ac"
        schedule={clock.schedule}
        source={RECORDING_FIXTURE}
      />
    );

    await waitFor(() =>
      expect(screen.getByTestId("terminal-recording-clock")).toHaveTextContent("0:00 / 2:29")
    );
    expect(screen.getByTestId("terminal-recording-toggle")).toHaveAccessibleName("Play");
  });

  it("Should advance the transport as recorded time passes", async () => {
    const clock = createManualClock();
    render(
      <TerminalRecordingPlayer
        autoPlay
        engineLoader={stubEngineLoader}
        recordingId="rec-9f21ac"
        schedule={clock.schedule}
        source={RECORDING_FIXTURE}
      />
    );

    await waitFor(() =>
      expect(screen.getByTestId("terminal-recording-toggle")).toHaveAccessibleName("Pause")
    );

    clock.advanceTo(149_000);

    await waitFor(() =>
      expect(screen.getByTestId("terminal-recording-clock")).toHaveTextContent("2:29 / 2:29")
    );
  });

  it("Should name the recording and where it came from when the caller knows", async () => {
    render(
      <TerminalRecordingPlayer
        engineLoader={stubEngineLoader}
        recordedAtLabel="12:47"
        recordingId="rec-9f21ac"
        retentionNote="kept for 30 days"
        source={RECORDING_FIXTURE}
        title="make gate"
      />
    );

    const player = await screen.findByTestId("terminal-recording-player");
    expect(player).toHaveTextContent("make gate");
    expect(player).toHaveTextContent("rec-9f21ac");
    expect(player).toHaveTextContent("12:47 · kept for 30 days");
  });

  it("Should close quietly when it is dismissed before the emulator arrives", async () => {
    const rejections: unknown[] = [];
    const onRejection = (event: PromiseRejectionEvent) => rejections.push(event.reason);
    window.addEventListener("unhandledrejection", onRejection);
    const clock = createManualClock();
    const { unmount } = render(
      <TerminalRecordingPlayer
        autoPlay
        engineLoader={() => new Promise(() => undefined)}
        recordingId="rec-9f21ac"
        schedule={clock.schedule}
        source={RECORDING_FIXTURE}
      />
    );

    // Playback does not await the parse, so a view that closes mid-write would
    // otherwise leave its rejection with nobody to catch it.
    clock.advanceTo(0);
    unmount();
    await new Promise(resolve => setTimeout(resolve, 0));

    expect(rejections).toEqual([]);
    window.removeEventListener("unhandledrejection", onRejection);
  });

  it("Should say a recording is unreadable instead of showing an empty screen", async () => {
    render(
      <TerminalRecordingPlayer
        engineLoader={stubEngineLoader}
        recordingId="rec-broken"
        source="not a recording"
      />
    );

    await waitFor(() => expect(screen.getByTestId("terminal-recording-error")).toBeInTheDocument());
  });

  it("Should let playback be paused from the transport", async () => {
    const clock = createManualClock();
    render(
      <TerminalRecordingPlayer
        autoPlay
        engineLoader={stubEngineLoader}
        recordingId="rec-9f21ac"
        schedule={clock.schedule}
        source={RECORDING_FIXTURE}
      />
    );
    await waitFor(() =>
      expect(screen.getByTestId("terminal-recording-toggle")).toHaveAccessibleName("Pause")
    );

    await userEvent.click(screen.getByTestId("terminal-recording-toggle"));

    expect(screen.getByTestId("terminal-recording-toggle")).toHaveAccessibleName("Play");
  });
});
