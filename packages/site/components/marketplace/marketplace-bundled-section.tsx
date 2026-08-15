import { Eyebrow } from "@compozy/ui";
import {
  BUNDLED_SPEC_CYCLE_PATH,
  bundledSkills,
  specCycleExtension,
} from "@/lib/marketplace-bundled";
import { BundledExtensionCard, BundledSkillCard } from "./marketplace-bundled-card";

/**
 * The runtime arrives with capabilities already installed, and the catalog cannot represent them:
 * `spec-cycle` is enrolled from the binary at first boot, and the bundled skills are compiled in. The
 * section therefore shows no install command — the honest action is to inspect what you already
 * have.
 */

function inventorySummary(): string {
  const { loops, skills, agents, tools } = specCycleExtension;
  return [
    `${loops.length} loops`,
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
        <BundledExtensionCard
          href={BUNDLED_SPEC_CYCLE_PATH}
          name={specCycleExtension.displayName}
          version={specCycleExtension.version}
          description={specCycleExtension.description}
          inventory={inventorySummary()}
          minCompozyVersion={specCycleExtension.minCompozyVersion}
          statusCommand={specCycleExtension.statusCommand}
        />
        {bundledSkills.map(skill => (
          <BundledSkillCard key={skill.name} name={skill.name} description={skill.description} />
        ))}
      </div>
    </section>
  );
}
