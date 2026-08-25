import { FolderTree } from "lucide-react";
import type { ReactNode } from "react";

import { Button, Empty, MonoId, Pill, Section } from "@compozy/ui";

import type {
  SettingsSkillSourcesModel,
  SkillSourceKey,
  SkillSourceKeyPosture,
} from "../hooks/use-settings-skill-sources";
import { SettingsSkillCustomSources } from "./settings-skill-custom-sources";
import { SettingsSkillSourceRow } from "./settings-skill-source-row";
import { SettingsInlineSaveControls } from "./settings-inline-save-controls";

interface SettingsSkillSourcesSectionProps {
  model: SettingsSkillSourcesModel;
}

const TEST_ID = "settings-page-skills-sources";

/**
 * Which folder conventions CompozyOS scans. Rows, counts, and folder states are
 * the daemon's measurement; the switches and the folder list are policy, so they
 * stay editable even when nothing could be measured.
 */
export function SettingsSkillSourcesSection({ model }: SettingsSkillSourcesSectionProps) {
  const { groups } = model;
  const anyOptionalOn = groups.presets.some(source => !source.alwaysOn && source.enabled);
  const defaultsOnly = !anyOptionalOn && model.customEntries.length === 0;

  return (
    <Section
      divided
      label="Sources"
      right={
        model.readOnly ? undefined : (
          <SettingsInlineSaveControls
            controlTestIdPrefix={TEST_ID}
            error={model.saveError}
            isDirty={model.isDirty}
            isSaving={model.isSaving}
            lastAppliedLabel={model.lastLabel}
            onReset={model.reset}
            onSave={model.save}
            saveLabel="Apply"
            testId={`${TEST_ID}-controls`}
          />
        )
      }
    >
      {model.readOnly ? (
        <p className="text-sm text-muted" data-testid={`${TEST_ID}-read-only`}>
          {model.readOnlyReason === "repository-profile"
            ? "This workspace projection follows the active profile and is read-only."
            : "Agent scope only supports disabled-skill tombstones. Sources stay user and workspace policy."}
        </p>
      ) : null}
      {(model.saveError ?? model.inheritError) ? (
        <p
          className="flex flex-wrap items-center gap-1.5 text-sm text-danger"
          data-testid={`${TEST_ID}-save-error`}
          role="alert"
        >
          <span>
            <b>Couldn&rsquo;t save.</b> {model.saveError ?? model.inheritError} Your changes are
            still here.
          </span>
          {model.saveErrorCode !== null ? (
            <MonoId className="text-faint" preserveCase value={model.saveErrorCode} />
          ) : null}
        </p>
      ) : null}
      <SourceKeyGroup model={model} posture={postureFor(model.postures, "sources")} title="Presets">
        <div
          className="overflow-hidden rounded-lg border border-line"
          data-testid={`${TEST_ID}-list`}
        >
          {groups.presets.map(source => (
            <SettingsSkillSourceRow
              disabled={model.readOnly}
              key={source.slug}
              onToggle={
                source.alwaysOn ? undefined : enabled => model.togglePreset(source.slug, enabled)
              }
              source={source}
            />
          ))}
        </div>
      </SourceKeyGroup>
      <SourceKeyGroup
        model={model}
        posture={postureFor(model.postures, "custom_sources")}
        title="Your folders"
      >
        <div className="overflow-hidden rounded-lg border border-line">
          <SettingsSkillCustomSources
            disabled={model.readOnly}
            entries={model.customEntries}
            onAdd={model.addCustom}
            onRemove={model.removeCustom}
            sources={groups.custom}
            validate={model.validateEntry}
          />
        </div>
      </SourceKeyGroup>
      {defaultsOnly ? (
        <Empty
          data-testid={`${TEST_ID}-defaults-only`}
          description="Only CompozyOS's built-in folders are on. Turn on a source above, or add your own folder."
          icon={FolderTree}
          title="Defaults only"
        />
      ) : null}
    </Section>
  );
}

function postureFor(
  postures: SkillSourceKeyPosture[] | null,
  key: SkillSourceKey
): SkillSourceKeyPosture | null {
  return postures?.find(posture => posture.key === key) ?? null;
}

/**
 * One config key. At workspace scope each key states whether it follows the
 * layer above and offers the single control that changes that.
 */
function SourceKeyGroup({
  model,
  posture,
  title,
  children,
}: {
  model: SettingsSkillSourcesModel;
  posture: SkillSourceKeyPosture | null;
  title: string;
  children: ReactNode;
}) {
  const testId = `${TEST_ID}-key-${posture?.key ?? title.toLowerCase().replace(/\s+/u, "-")}`;
  return (
    <div className="flex min-w-0 flex-col gap-2" data-testid={testId}>
      {posture !== null ? (
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <span className="text-sm font-medium text-fg">{title}</span>
          <Pill size="xs" data-testid={`${testId}-posture`}>
            {posture.inherited ? "inherited" : "custom for this workspace"}
          </Pill>
          {model.readOnly ? null : posture.inherited && !posture.armed ? (
            <Button
              data-testid={`${testId}-customize`}
              onClick={() => model.customize(posture.key)}
              size="sm"
              type="button"
              variant="ghost"
            >
              Customize
            </Button>
          ) : (
            <Button
              data-testid={`${testId}-use-inherited`}
              disabled={model.inheritPendingKey !== null}
              onClick={() => model.useInherited(posture.key)}
              size="sm"
              type="button"
              variant="ghost"
            >
              {model.inheritPendingKey === posture.key ? "Applying…" : "Use inherited"}
            </Button>
          )}
        </div>
      ) : null}
      {children}
    </div>
  );
}
