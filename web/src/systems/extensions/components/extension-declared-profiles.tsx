import { CircleDot, TriangleAlert } from "lucide-react";

import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
  Button,
  Pill,
} from "@compozy/ui";

import type { ExtensionEntry } from "../types";
import { openProfileDialog, ProfileGlyph } from "@/systems/profiles";

export function ExtensionDeclaredProfiles({ extension }: { extension: ExtensionEntry }) {
  const profiles = extension.declared_profiles ?? [];
  const placements = extension.placements ?? [];
  const dormant = extension.dormant_placements ?? [];
  if (profiles.length === 0 && placements.length === 0) return null;

  return (
    <div className="divide-y divide-line-soft" data-testid="extension-declared-profiles">
      {profiles.map(profile => (
        <div className="flex min-h-11 items-center gap-2.5 px-4 py-2.5" key={profile.name}>
          <ProfileGlyph
            decorative
            name={profile.name}
            needsSetup={profile.needs_setup === true}
            size="sm"
          />
          <span className="min-w-0 flex-1 truncate text-sm text-fg">{profile.name}</span>
          {!profile.exists ? <Pill tone="info">Dormant</Pill> : null}
          {profile.needs_setup ? (
            <Pill tone="warning">
              <TriangleAlert aria-hidden="true" className="size-3" />
              Needs setup
            </Pill>
          ) : null}
        </div>
      ))}

      {dormant.map(placement => (
        <div
          className="flex min-h-11 items-center gap-2.5 px-4 py-2.5"
          data-testid={`extension-dormant-${placement.profile}`}
          key={`${placement.kind}:${placement.resource}:${placement.profile}`}
        >
          <CircleDot aria-hidden="true" className="size-3.5 shrink-0 text-info" />
          <span className="min-w-0 flex-1 text-sm text-muted">
            {placement.resource} · {placement.profile}
          </span>
          <Button
            onClick={() =>
              openProfileDialog({ flow: "create", profile: placement.profile || undefined })
            }
            size="sm"
            type="button"
            variant="neutral"
          >
            Create profile
          </Button>
        </div>
      ))}

      {placements.length > 0 ? (
        <Accordion className="px-4" defaultValue={[]}>
          <AccordionItem value="placements">
            <AccordionTrigger className="text-xs text-muted">Placement matrix</AccordionTrigger>
            <AccordionContent>
              <dl className="divide-y divide-line-soft pb-2">
                {placements.map(placement => (
                  <div
                    className="grid grid-cols-[minmax(0,1fr)_auto] gap-3 py-2 text-xs"
                    key={`${placement.kind}:${placement.resource}:${placement.profile ?? "global"}`}
                  >
                    <dt className="min-w-0 truncate text-fg">
                      {placement.kind} · {placement.resource}
                    </dt>
                    <dd className="text-muted">{placement.profile || "Machine-wide"}</dd>
                  </div>
                ))}
              </dl>
            </AccordionContent>
          </AccordionItem>
        </Accordion>
      ) : null}
    </div>
  );
}
