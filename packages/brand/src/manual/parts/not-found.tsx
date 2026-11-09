// Unknown URL: a calm dead end with the way back to Foundations (US-001.EC-1).
import { Link } from "@tanstack/react-router";

import { Visual } from "../art/visual";
import { VERSION } from "../chapters";

export function NotFound() {
  return (
    <main data-not-found className="band-ink relative isolate min-h-screen overflow-hidden">
      <Visual name="field" className="-z-10 opacity-60" />
      <div className="absolute inset-0 -z-10 bg-[linear-gradient(90deg,var(--color-rail)_0%,color-mix(in_oklab,var(--color-rail)_80%,transparent)_40%,transparent_75%)]" />
      <div className="wrap grid min-h-screen content-center gap-6 py-24">
        <p className="m-0 mono-label tracking-[0.12em] text-(--b-muted)">
          404 · brand manual {VERSION}
        </p>
        <h1 className="m-0 max-w-[14ch] text-display">
          This page is <em>not</em> in the manual.
        </h1>
        <p className="m-0 max-w-[48ch] text-lead text-(--b-muted)">
          The address does not match a chapter. Start again from Foundations.
        </p>
        <p className="m-0">
          <Link
            to="/"
            className="inline-flex min-h-11 items-center rounded-[10px] bg-(--color-brand-volt) px-5 text-[15px] font-medium text-(--color-fg-strong) no-underline hover:bg-(--color-brand-hover)"
          >
            Go to Foundations
          </Link>
        </p>
      </div>
    </main>
  );
}
