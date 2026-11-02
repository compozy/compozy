/** The link dialog over the session it links to; every direction opens this same dialog. */
import { useState } from "react";

import { SessionPullRequestChip } from "@/systems/session/components/session-pull-request-chip";
import type { SessionLink } from "@/systems/session/mocks/session-links-fixtures";

import type { LinkDialogPreset } from "../session-links-shared/link-dialog";
import { leadLink } from "../session-links-shared/link-model";
import { LinkStage } from "../session-links-shared/link-stage";
import { linkScenario } from "../session-links-shared/scenarios";
import { useLinks } from "../session-links-shared/use-links";

export function DialogSurface({
  linked,
  preset,
}: {
  linked: readonly SessionLink[];
  preset: LinkDialogPreset;
}) {
  const scenario = { ...linkScenario("mixed-rest"), links: linked };
  const { links, add } = useLinks(linked);
  const [open, setOpen] = useState(true);
  const lead = leadLink(links);
  return (
    <LinkStage
      dialog={preset}
      dialogOpen={open}
      links={links}
      onAdd={add}
      onDialogOpenChange={setOpen}
      scenario={scenario}
      status={lead ? <SessionPullRequestChip controllable facts={lead.facts} /> : null}
    />
  );
}
