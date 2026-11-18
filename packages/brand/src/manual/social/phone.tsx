// Phone frame with an X-style feed, drawn in HTML. The post carries a kit template as its image.
import { cn } from "@compozy/ui/utils";

import { Frame } from "../kit-frame";

interface PhoneProps {
  id?: string;
  name?: string;
  handle?: string;
  text?: string;
  className?: string;
}

export function Phone({
  id = "release-card",
  name = "Compozy",
  handle = "@compozy",
  text = "Compozy 0.6 is out. Always-on agents check in on their own and call you only when they need a decision.",
  className,
}: PhoneProps) {
  return (
    <div
      className={cn(
        "mx-auto w-full max-w-[340px] rounded-[44px] border border-line-strong bg-black p-2.5 shadow-[0_0_0_1px_var(--color-canvas)]",
        className
      )}
    >
      <div className="overflow-hidden rounded-[36px] bg-black text-fg">
        <div className="flex items-center justify-between px-6 pt-3 pb-2 font-sans text-[12px] font-semibold">
          <span>9:41</span>
          <span className="h-5 w-20 rounded-full bg-canvas" />
          <span>5G</span>
        </div>
        <div className="flex border-b border-line text-[13px]">
          <span className="flex-1 border-b-2 border-(--color-brand-volt) py-2.5 text-center font-semibold">
            For you
          </span>
          <span className="flex-1 py-2.5 text-center text-muted-foreground">Following</span>
        </div>
        <div className="flex gap-2.5 border-b border-line px-3.5 py-3">
          <img
            src="/brand/png/compozy-app-icon-128h.png"
            alt=""
            className="size-9 shrink-0 rounded-full bg-rail object-cover"
          />
          <div className="min-w-0 flex-1">
            <p className="m-0 truncate text-[13px]">
              <b>{name}</b> <span className="text-muted-foreground">{handle} · 2h</span>
            </p>
            <p className="m-0 mt-1 text-[13px] leading-[1.4]">{text}</p>
            <div className="mt-2.5 overflow-hidden rounded-[14px] border border-line">
              <Frame id={id} caption={false} />
            </div>
            <div className="mt-2.5 flex justify-between pr-6 font-mono text-[11px] text-muted-foreground">
              <span>↩ 24</span>
              <span>⇄ 61</span>
              <span>♡ 318</span>
              <span>▤ 21k</span>
            </div>
          </div>
        </div>
        <div className="flex gap-2.5 px-3.5 py-3 opacity-60">
          <span className="size-9 shrink-0 rounded-full bg-line" />
          <div className="flex-1">
            <span className="block h-2.5 w-1/2 rounded bg-line" />
            <span className="mt-2 block h-2.5 w-full rounded bg-line" />
            <span className="mt-1.5 block h-2.5 w-4/5 rounded bg-line" />
          </div>
        </div>
      </div>
    </div>
  );
}
