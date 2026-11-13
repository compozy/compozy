import { AgentAvatar } from "@compozy/ui";

import { useBootRunner } from "@/drafts/desktop-boot/hooks/use-boot-runner";

/**
 * The waiting game in the lead column's empty ink: an always-on agent's face runs the point
 * line and hops the stacks. It plays itself; Space or a click takes over. A miss shows the
 * Failed face and hands the run to the next agent. The states are the product's own: Working
 * while it runs, Failed on a miss, Paused until it starts.
 */
export function BootRunner({ autoplay }: { autoplay: boolean }) {
  const { runner, ground, blocks, onJump, still, mood, score, agent, slots } = useBootRunner({
    autoplay,
  });
  return (
    <div className="boot__runner" data-mood={mood}>
      <p className="boot__runner-hud">
        <span>
          {agent}
          {still ? null : <i>{mood === "paused" ? "Press Space to play" : "Space to jump"}</i>}
        </span>
        <span>{String(score).padStart(3, "0")}</span>
      </p>
      {/* Reduced motion stops the game, so the track offers no jump it cannot perform. */}
      <button
        className="boot__runner-track"
        type="button"
        aria-label="Jump"
        disabled={still}
        onClick={onJump}
      >
        <span ref={runner} className="boot__runner-agent">
          <AgentAvatar
            key={agent}
            appearance={{ avatar: "generated", seed: agent }}
            label={agent}
            size={32}
            state={mood}
            surfaceClassName="ring-rail"
          />
        </span>
        {Array.from({ length: slots }, (_, index) => (
          <span
            key={index}
            ref={element => {
              blocks.current[index] = element;
            }}
            className="boot__runner-block"
          />
        ))}
        <span className="boot__runner-ground">
          <span ref={ground} />
        </span>
      </button>
    </div>
  );
}
