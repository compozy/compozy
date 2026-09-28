import { FileCode2, Layers, MonitorCog } from "lucide-react";
import Image from "next/image";

const cardBase =
  "group relative isolate min-w-0 overflow-hidden rounded-diagram border border-line bg-rail p-7 transition-colors hover:border-accent/40 sm:p-8 xl:p-10";

const labelBase = "eyebrow font-semibold! mb-5 flex items-center gap-3 text-accent";

const imageBase = "h-full w-full select-none opacity-95";

export function BentoSection() {
  return (
    <section
      id="runtime-map"
      aria-label="CompozyOS runtime map"
      className="scroll-mt-24 border-y border-line bg-rail px-4 py-6 sm:px-5 md:py-10 lg:px-5 lg:py-24"
    >
      <div
        data-testid="bento-grid"
        className="mx-auto grid w-full max-w-300 gap-4 md:grid-cols-2 lg:grid-cols-3"
      >
        <RuntimeCard />
        <MemoryCard />
        <ExtensibilityCard />
      </div>
    </section>
  );
}

function RuntimeCard() {
  return (
    <article
      data-testid="bento-runtime"
      className={`${cardBase} min-h-135 md:min-h-140 lg:col-span-1`}
    >
      <div className="absolute inset-x-0 bottom-0 top-[0%] pointer-events-none">
        <Image
          src="/images/bento-illustrations/os-v2.png"
          alt="CompozyOS shell routing several application windows through a central control surface."
          fill
          loading="lazy"
          decoding="async"
          sizes="(min-width: 1024px) 50vw, 100vw"
          quality={90}
          className={`${imageBase} object-cover`}
        />
      </div>
      <div className="site-bento-overlay-runtime pointer-events-none absolute inset-0" />

      <div className="relative z-10 max-w-84">
        <div className={labelBase}>
          <MonitorCog className="size-4" />
          <span>OS Shell</span>
        </div>
        <h2
          aria-label="Batteries included. Every window managed."
          className="font-display text-site-bento-lg font-normal leading-tight text-fg sm:text-site-bento-xl xl:text-site-bento-2xl"
        >
          Batteries included.
          <br />
          <span className="text-accent">Every window managed.</span>
        </h2>
        <span className="mt-5 block h-px w-8 bg-accent" aria-hidden="true" />
      </div>
    </article>
  );
}

function MemoryCard() {
  return (
    <article data-testid="bento-memory" className={`${cardBase} min-h-97.5 lg:col-span-1`}>
      <div className="absolute inset-x-0 bottom-0 top-[18%] pointer-events-none">
        <Image
          src="/images/bento-illustrations/memory-v2.png"
          alt="Skill document carrying deployment intent into CompozyOS memory."
          fill
          decoding="async"
          sizes="(min-width: 1024px) 33vw, 100vw"
          quality={90}
          className={`${imageBase} object-cover object-[50%_80%]`}
        />
      </div>
      <div className="site-bento-overlay-memory pointer-events-none absolute inset-0" />

      <div className="relative z-10 max-w-68">
        <div className={labelBase}>
          <FileCode2 className="size-4" />
          <span>Memory</span>
        </div>
        <h2
          aria-label="Memory that compounds."
          className="font-display text-site-bento-sm font-normal leading-tight text-fg sm:text-site-bento-lg xl:text-4xl"
        >
          Memory that
          <br />
          <span className="text-accent">compounds.</span>
        </h2>
      </div>
    </article>
  );
}

function ExtensibilityCard() {
  return (
    <article data-testid="bento-extensibility" className={`${cardBase} min-h-97.5 lg:col-span-1`}>
      <div className="absolute inset-0 -bottom-30 pointer-events-none">
        <Image
          src="/images/bento-illustrations/extensibility-v2.png"
          alt="CompozyOS daemon device with five pluggable extension cartridges — hooks, skills, tools, automation, extensions — snapping into the runtime."
          fill
          decoding="async"
          sizes="(min-width: 1024px) 33vw, 100vw"
          quality={90}
          className={`${imageBase} object-cover object-[10%_10%]`}
        />
      </div>
      <div className="site-bento-overlay-extensibility pointer-events-none absolute inset-0" />

      <div className="relative z-10 max-w-84">
        <div className={labelBase}>
          <Layers className="size-4" />
          <span>Extensibility</span>
        </div>
        <h2
          aria-label="Every layer. Pluggable."
          className="font-display text-site-bento-sm font-normal leading-tight text-fg sm:text-site-bento-lg xl:text-4xl"
        >
          Every layer.
          <br />
          <span className="text-accent">Pluggable.</span>
        </h2>
      </div>
    </article>
  );
}
