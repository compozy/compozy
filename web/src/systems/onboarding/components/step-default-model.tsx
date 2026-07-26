import { Fragment } from "react";
import { Check, KeyRound } from "lucide-react";

import { Eyebrow, Field, FieldLabel, Input, RadioCard, Spinner } from "@agh/ui";
import { RuntimeSelector } from "@/systems/runtime";

import type { OnboardingDefaultModelApi } from "../hooks/use-onboarding-default-model";
import type { OnboardingModelFact } from "../lib/model-facts";
import { ONBOARDING_SUMMARY_ID } from "../lib/onboarding-summary";
import type { OnboardingAuthMode } from "../stores/use-onboarding-draft-store";

const AUTH_OPTIONS: { mode: OnboardingAuthMode; title: string; description: string }[] = [
  {
    mode: "native_cli",
    title: "Use the provider CLI",
    description: "Reuse the sign-in already on this machine. Nothing to paste.",
  },
  {
    mode: "bound_secret",
    title: "Use an API key",
    description: "Bind a key from an environment variable, or paste one now.",
  },
];

function ModelFacts({ facts }: { facts: OnboardingModelFact[] }) {
  if (facts.length === 0) return null;
  return (
    <p
      data-testid="onboarding-model-facts"
      className="mt-2.5 flex flex-wrap items-center gap-x-1.5 gap-y-1 text-form-hint text-faint"
    >
      {facts.map((fact, index) => (
        <Fragment key={fact.id}>
          {index > 0 ? (
            <span aria-hidden="true" className="size-0.5 flex-none rounded-full bg-faint" />
          ) : null}
          <span>
            {fact.value === null ? null : (
              <span className="font-medium text-muted">{fact.value} </span>
            )}
            {fact.label}
          </span>
        </Fragment>
      ))}
    </p>
  );
}

interface StepDefaultModelProps {
  model: OnboardingDefaultModelApi;
}

export function StepDefaultModel({ model }: StepDefaultModelProps) {
  return (
    <div className="mt-5.5 flex flex-col gap-5.5" data-testid="onboarding-step-default-model">
      <section>
        <Eyebrow id="onboarding-runtime-label" className="mb-2.5 block text-subtle">
          Runtime
        </Eyebrow>
        {model.providersLoading ? (
          <div className="flex items-center gap-2 text-small-body text-muted">
            <Spinner /> Loading providers…
          </div>
        ) : model.providersError ? (
          <p className="text-small-body text-danger" role="alert">
            {model.providersError}
          </p>
        ) : (
          <RuntimeSelector
            value={model.runtimeValue}
            onChange={model.onRuntimeChange}
            providers={model.runtimeProviders}
            models={model.runtimeModels}
            loading={model.catalogLoading}
            catalogLoaded={model.catalogLoaded}
            refreshing={model.catalogRefreshing}
            onRefreshCatalog={model.onRefreshCatalog}
            disabled={model.runtimeProviders.length === 0}
            ariaLabelledby="onboarding-runtime-label"
            triggerTestId="onboarding-runtime-select"
          />
        )}
        <ModelFacts facts={model.facts} />
        {model.catalogError ? (
          <p className="mt-2 text-form-hint text-danger" role="alert">
            {model.catalogError}
          </p>
        ) : null}
      </section>

      <section>
        <Eyebrow id="onboarding-auth-label" className="mb-2.5 block text-subtle">
          How AGH signs in
        </Eyebrow>
        <div
          role="radiogroup"
          aria-labelledby="onboarding-auth-label"
          className="grid grid-cols-1 gap-2.5 sm:grid-cols-2"
        >
          {AUTH_OPTIONS.map(option => {
            const selected = option.mode === model.authMode;
            return (
              <RadioCard
                key={option.mode}
                selected={selected}
                onSelect={() => model.onAuthModeChange(option.mode)}
                title={option.title}
                titleClassName="min-w-0 flex-1"
                description={option.description}
                badge={
                  <Check
                    aria-hidden="true"
                    className={selected ? "size-3.5 text-fg-strong" : "size-3.5 opacity-0"}
                  />
                }
                className="p-3"
                data-testid={`onboarding-auth-${option.mode}`}
              />
            );
          })}
        </div>

        {model.authMode === "bound_secret" ? (
          <div className="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
            <Field>
              <FieldLabel>Environment variable</FieldLabel>
              <Input
                value={model.envVar}
                spellCheck={false}
                autoComplete="off"
                onChange={event => model.onEnvVarChange(event.currentTarget.value)}
                placeholder="PROVIDER_API_KEY"
                aria-invalid={model.missingEnvVar}
                aria-describedby={model.missingEnvVar ? ONBOARDING_SUMMARY_ID : undefined}
                className="font-mono"
                data-testid="onboarding-env-var"
              />
            </Field>
            <Field>
              <FieldLabel>
                API key <span className="font-normal text-faint">(optional)</span>
              </FieldLabel>
              <div className="relative">
                <KeyRound className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-subtle" />
                <Input
                  type="password"
                  value={model.apiKey}
                  spellCheck={false}
                  autoComplete="off"
                  onChange={event => model.onApiKeyChange(event.currentTarget.value)}
                  placeholder="sk-…"
                  className="pl-8"
                  data-testid="onboarding-api-key"
                />
              </div>
            </Field>
          </div>
        ) : null}
      </section>
    </div>
  );
}
