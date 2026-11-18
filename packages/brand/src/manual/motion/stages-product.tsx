// Signature stages built from product state: a face changing state, a card crossing the board,
// a handoff to another harness, the dock answering. Each renders its final frame; demos.ts plays it.
import { AgentAvatar } from "@compozy/ui/components/custom/agent-avatar";

const foot = "absolute bottom-3 left-4 m-0 mono-label text-[11px] text-muted-foreground";

export function FaceStateStage() {
  return (
    <>
      <div className="absolute inset-[14%_8%_auto] flex items-center justify-between rounded-[10px] border border-line bg-canvas px-4 py-3 font-mono text-[12.5px]">
        <span className="flex items-center gap-4">
          <span className="relative block size-[42px]">
            <span data-fs="working" className="absolute inset-0 opacity-0">
              <AgentAvatar
                appearance={{ avatar: "generated", seed: "checkout-lead" }}
                label="checkout-lead"
                state="working"
                size={42}
              />
            </span>
            <span data-fs="needs" className="absolute inset-0">
              <AgentAvatar
                appearance={{ avatar: "generated", seed: "checkout-lead" }}
                label="checkout-lead"
                state="needs-you"
                size={42}
              />
            </span>
          </span>
          <span className="grid gap-1">
            <span className="text-fg">checkout-lead</span>
            <span className="relative block h-4 w-[96px]">
              <span
                data-fl="working"
                className="absolute inset-0 text-(--color-brand-cyan) opacity-0"
              >
                ● working
              </span>
              <span data-fl="needs" className="absolute inset-0 text-warning">
                ▲ needs you
              </span>
            </span>
          </span>
        </span>
        <span className="text-muted-foreground">claude code</span>
      </div>
      <p className={foot}>always-on agent · working → needs you</p>
    </>
  );
}

export function CardMoveStage() {
  return (
    <>
      <div className="absolute inset-[14%_6%_20%] grid grid-cols-3 gap-2 mono-label text-[10.5px] tracking-[0.08em] text-muted-foreground">
        {["Working", "Needs you", "In review"].map(c => (
          <div key={c} className="rounded-[8px] border border-line bg-canvas p-2">
            {c}
          </div>
        ))}
      </div>
      <div className="absolute top-[30%] left-[6%] h-[22%] w-[29%] rounded-[8px] border border-line-strong bg-elevated p-2 font-mono text-[11px] opacity-60">
        <span className="text-muted-foreground">fix-flaky-tests</span>
      </div>
      <div
        data-card-move
        className="absolute top-[30%] left-[37.4%] h-[22%] w-[29%] rounded-[8px] border border-warning bg-elevated p-2 font-mono text-[11px] shadow-[0_0_18px_color-mix(in_oklab,var(--color-warning)_25%,transparent)]"
      >
        <span className="text-fg">migrate-db</span>
        <br />
        <span className="text-warning">▲ needs you</span>
      </div>
      <p className={foot}>board · by who has the next step</p>
    </>
  );
}

export function HandoffStage() {
  const box =
    "absolute top-[28%] h-[44%] w-[34%] rounded-[10px] border border-(--color-brand-cyan) bg-canvas p-3 font-mono text-[11px]";
  return (
    <>
      <div data-a className={`${box} left-[6%] opacity-35`}>
        <span className="text-(--color-brand-cyan)">● claude code</span>
        <br />
        <span className="text-muted-foreground">checkout-lead · context</span>
      </div>
      <div data-b className={`${box} right-[6%]`}>
        <span className="text-(--color-brand-cyan)">● codex</span>
        <br />
        <span className="text-muted-foreground">continues · same worktree</span>
      </div>
      <div className="absolute top-1/2 right-[40%] left-[40%] h-px bg-line-strong">
        <span
          data-pk
          className="absolute -top-[5px] left-full block size-[11px] -translate-x-1/2 rounded-full bg-(--color-brand-cyan) opacity-0 shadow-[0_0_14px_var(--color-brand-cyan)]"
        />
      </div>
    </>
  );
}

export function DockAnswerStage() {
  const action = "rounded-full border border-line-strong px-3 py-1.5";
  return (
    <>
      <div className="absolute inset-x-[7%] top-[18%] h-[46%] rounded-[12px] border border-line-strong bg-canvas p-4 font-mono text-[12px]">
        <div data-dk="ask" className="absolute inset-4 opacity-0">
          <p className="m-0 text-fg">payments-codex asks to run the migration on staging</p>
          <div className="mt-4 flex gap-2">
            <span data-allow className={`${action} text-fg`}>
              Allow once
            </span>
            <span className={`${action} text-muted-foreground`}>Always allow</span>
            <span className={`${action} text-muted-foreground`}>Deny</span>
          </div>
        </div>
        <div data-dk="done" className="absolute inset-4">
          <p className="m-0 text-success">✓ allowed once</p>
          <p className="m-0 mt-3 text-(--color-brand-cyan)">● payments-codex · running</p>
        </div>
      </div>
      <p className={foot}>permission dock · ask → answer</p>
    </>
  );
}
