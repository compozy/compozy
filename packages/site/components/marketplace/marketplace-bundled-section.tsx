import { Eyebrow } from "@compozy/ui";
import { bundledExtensions, bundledSkills, type BundledExtension } from "@/lib/marketplace-bundled";
import { BundledExtensionCard, BundledSkillCard } from "./marketplace-bundled-card";

function inventorySummary({ loops, skills, agents, tools }: BundledExtension): string {
  return [
    `${loops.length} ${loops.length === 1 ? "loop" : "loops"}`,
    `${skills.length} skills`,
    `${agents.length} agents`,
    `${tools.length} ${tools.length === 1 ? "tool" : "tools"}`,
  ].join(" · ");
}

export function MarketplaceBundledSection() {
  return (
    <section
      aria-labelledby="bundled"
      className="grid gap-6 border-t border-line pt-10 lg:grid-cols-[minmax(0,17rem)_minmax(0,1fr)] lg:gap-10"
    >
      <div>
        <Eyebrow className="text-subtle">Already installed</Eyebrow>
        <h2 id="bundled" className="mt-2 text-xl font-semibold tracking-[-0.015em] text-fg">
          Ships with the runtime
        </h2>
        <p className="mt-3 text-small-body leading-relaxed text-muted">
          These are compiled into the compozy binary and enrolled the first time the daemon starts.
          They carry no catalog entry and no install command because there is nothing to fetch —
          upgrading the runtime upgrades them.
        </p>
      </div>

      <div className="flex flex-col gap-4">
        {bundledExtensions.map(extension => (
          <BundledExtensionCard
            key={extension.name}
            href={extension.path}
            name={extension.displayName}
            version={extension.version}
            description={extension.description}
            inventory={inventorySummary(extension)}
            minCompozyVersion={extension.minCompozyVersion}
            statusCommand={extension.statusCommand}
          />
        ))}
        {bundledSkills.map(skill => (
          <BundledSkillCard key={skill.name} name={skill.name} description={skill.description} />
        ))}
      </div>
    </section>
  );
}
