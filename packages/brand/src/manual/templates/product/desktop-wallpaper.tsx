// Desktop wallpaper, 2880×1800 (MacBook Pro at 2×): Atmosphere horizon, the one-colour symbol and
// one mono line bottom-left, above the dock line. Also the empty desktop behind the Compozy shell.
import { Visual } from "../../art/visual";
import { cq, ink, monoUp } from "../styles";

export function DesktopWallpaper({ meta = "compozy · always-on" }: { meta?: string }) {
  return (
    <div className={ink}>
      <Visual name="horizon" x={0.7} y={0.5} />
      <div
        aria-hidden
        className="absolute inset-0"
        style={{
          backgroundImage:
            "radial-gradient(120% 90% at 0% 100%, color-mix(in oklab, var(--color-rail) 95%, transparent) 0%, transparent 50%)",
        }}
      />
      <div
        className="absolute flex items-center"
        style={{ left: cq(4.2), bottom: cq(6.7), gap: cq(1) }}
      >
        <img
          src="/brand/svg/compozy-symbol-mono-white.svg"
          alt=""
          className="w-auto opacity-90"
          style={{ height: cq(2.2) }}
        />
        <p
          data-field="meta"
          className={`${monoUp} text-muted-foreground`}
          style={{ fontSize: ".75cqi", letterSpacing: ".12em" }}
        >
          {meta}
        </p>
      </div>
    </div>
  );
}
