import { Eyebrow } from "@compozy/ui";
import { HeroVisual } from "@/components/landing/hero-visual";
import type { DesktopRelease } from "@/lib/release/desktop-downloads";
import { PrimaryDownload } from "./primary-download";

export function DownloadHero({ release }: { release: DesktopRelease }) {
  return (
    <section className="relative overflow-hidden border-b border-line px-4 pt-10 pb-16 md:pt-16 md:pb-24">
      <div
        className="pointer-events-none absolute inset-0 bg-size-[100%_auto] bg-position-[0%_0%] bg-no-repeat opacity-20 mix-blend-screen"
        style={{ backgroundImage: "url('/hero-bg.webp')" }}
        aria-hidden="true"
      />

      <div className="relative mx-auto grid max-w-site-layout-width gap-12 lg:grid-cols-[minmax(0,6fr)_minmax(0,5fr)] lg:items-center lg:gap-10">
        <div className="lg:pr-2">
          <Eyebrow className="flex items-center gap-3 text-muted-foreground">
            <span className="text-brand">Desktop app</span>
            <span className="h-px w-10 bg-line" />
            <span>{release.status === "ready" ? release.tag : "Latest release"}</span>
          </Eyebrow>

          <h1 className="mt-6 max-w-[16ch] font-display text-site-hero leading-none font-normal tracking-tight text-fg">
            CompozyOS, on your desktop.
          </h1>

          <p className="mt-6 max-w-[58ch] text-base leading-relaxed text-muted-foreground md:text-lg">
            The app carries its own runtime. Open it and it attaches to the daemon already on your
            machine, or installs and starts the one inside the app. Your workspaces, sessions, and
            automations, in one native window.
          </p>

          <div className="mt-8">
            <PrimaryDownload release={release} />
          </div>
        </div>

        <div className="relative">
          <HeroVisual className="mx-auto w-full max-w-140 lg:mx-0 lg:w-[120%] lg:max-w-none" />
        </div>
      </div>
    </section>
  );
}
