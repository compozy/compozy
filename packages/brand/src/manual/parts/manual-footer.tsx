import { Visual } from "../art/visual";
import { VERSION } from "../chapters";

export function ManualFooter() {
  return (
    <footer className="relative isolate overflow-hidden bg-black text-sm text-muted-foreground">
      <Visual name="horizon" y={0.55} className="-z-10 opacity-70" />
      <div className="wrap grid gap-10 pt-20 pb-12">
        <p className="m-0 max-w-[16ch] font-display text-[clamp(36px,4.6vw,64px)] leading-[1.02] tracking-[-0.03em] text-fg">
          Keep every agent <em>running</em>.
        </p>
        <div className="flex flex-wrap items-center gap-x-8 gap-y-3">
          <img
            src="/brand/svg/compozy-lockup-dark.svg"
            alt="Compozy"
            className="h-6 w-auto"
            width="150"
            height="24"
          />
          <span>compozy.com · brand manual {VERSION}</span>
        </div>
      </div>
    </footer>
  );
}
