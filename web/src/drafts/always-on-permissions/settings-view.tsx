/** S15: the general control, in the real Settings group. */
import { Settings } from "@compozy/ui/icons";
import { useState } from "react";

import { PageContent } from "@compozy/ui";

import { SettingsChoiceGroup } from "@/systems/settings/components/settings-choice-group";
import { SettingsGroup } from "@/systems/settings/components/settings-group";

import { DraftShell } from "../always-on-agents-shared/draft-shell";
import { LEVELS, type PermissionLevel } from "./levels";

const OPTIONS = LEVELS.map(level => ({
  value: level.value,
  name: level.label,
  description: level.description,
}));

export function SettingsPermissions({ initial }: { initial: PermissionLevel }) {
  const [mode, setMode] = useState(initial);
  return (
    <DraftShell app="dashboard" glyph={<Settings aria-hidden="true" />} title="Settings">
      <PageContent className="@container/settings mx-auto max-w-4xl py-8">
        <SettingsGroup title="Permissions">
          <SettingsChoiceGroup
            ariaLabel="Permission mode"
            onChange={setMode}
            options={OPTIONS}
            value={mode}
          />
        </SettingsGroup>
      </PageContent>
    </DraftShell>
  );
}
