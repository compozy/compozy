/** One agent CLI terminal from the pile in the problem section. */
import type { Pane } from "@/components/landing/story-data";

export function TerminalPane({ pane }: { pane: Pane }) {
  return (
    <div className="overflow-hidden rounded-[16px] border border-site-light-200 bg-[#0d0d0d] shadow-[0_40px_90px_-10px_#000,0_0_0_1px_rgba(0,0,0,0.6)]">
      <div className="flex items-center gap-1.5 border-b border-site-light-100 px-3.5 py-2.5">
        {[0, 1, 2].map(dot => (
          <span key={dot} className="size-2.5 rounded-full bg-site-light-200" />
        ))}
        <span className="ml-2 font-mono text-[12.5px] text-muted-foreground">{pane.name}</span>
      </div>
      <div className="p-5 font-mono text-[14.5px] leading-[1.7] text-fg/70">
        <div className="text-fg">$ {pane.name}</div>
        {pane.lines.map(line => (
          <div key={line}>{line}</div>
        ))}
      </div>
    </div>
  );
}
