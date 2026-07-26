import { SettingsFieldRow, SettingsGroup, SettingsNumberInput } from "@/systems/settings";
import { Input } from "@agh/ui";
import { type ValidatedSectionProps, TEST_PREFIX } from "./-memory-settings-types";

export function ControllerSection({
  draft,
  setDraft,
  validationErrors,
  setValidationError,
}: ValidatedSectionProps) {
  const allowOrigins = draft.controller.policy.allow_origins.join(", ");
  return (
    <SettingsGroup
      title="Write controller"
      description="lexical/entity-only ADD / UPDATE / DELETE / NOOP / REJECT pipeline"
    >
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-controller-mode`}
        label="Controller mode"
        description="hybrid uses rules with an LLM tiebreaker; rules and llm pin a single strategy"
        control={
          <Input
            className="w-40 font-mono"
            data-testid={`${TEST_PREFIX}-controller-mode-input`}
            value={draft.controller.mode}
            placeholder="hybrid"
            onChange={event =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  controller: { ...current.controller, mode: event.target.value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-controller-max-latency`}
        label="Max latency"
        description="Hard deadline before the controller falls back to default-op"
        control={
          <Input
            className="w-32 font-mono"
            data-testid={`${TEST_PREFIX}-controller-max-latency-input`}
            value={draft.controller.max_latency}
            placeholder="300ms"
            onChange={event =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  controller: { ...current.controller, max_latency: event.target.value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-controller-default-op`}
        label="Default op on fail"
        description="Decision used when the controller bails (e.g. timeout, schema drift)"
        control={
          <Input
            className="w-32 font-mono"
            data-testid={`${TEST_PREFIX}-controller-default-op-input`}
            value={draft.controller.default_op_on_fail}
            placeholder="noop"
            onChange={event =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  controller: {
                    ...current.controller,
                    default_op_on_fail: event.target.value,
                  },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-controller-policy-max-content`}
        label="Max content chars"
        description="Per-candidate body cap enforced before the controller decides"
        error={validationErrors.policyMaxContentChars ?? undefined}
        control={
          <SettingsNumberInput
            min={0}
            className="w-32"
            data-testid={`${TEST_PREFIX}-controller-policy-max-content-input`}
            value={draft.controller.policy.max_content_chars}
            onValidityChange={setValidationError("policyMaxContentChars")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  controller: {
                    ...current.controller,
                    policy: { ...current.controller.policy, max_content_chars: value },
                  },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-controller-policy-max-writes`}
        label="Max writes per minute"
        description="Soft rate limit applied at controller entry"
        error={validationErrors.policyMaxWritesPerMin ?? undefined}
        control={
          <SettingsNumberInput
            min={0}
            className="w-32"
            data-testid={`${TEST_PREFIX}-controller-policy-max-writes-input`}
            value={draft.controller.policy.max_writes_per_min}
            onValidityChange={setValidationError("policyMaxWritesPerMin")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  controller: {
                    ...current.controller,
                    policy: { ...current.controller.policy, max_writes_per_min: value },
                  },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-controller-policy-allow-origins`}
        label="Allowed origins"
        description="Read-only roster of write origins permitted by this build"
        control={
          <Input
            readOnly
            className="w-full font-mono"
            data-testid={`${TEST_PREFIX}-controller-policy-allow-origins-input`}
            value={allowOrigins}
          />
        }
      />
    </SettingsGroup>
  );
}
