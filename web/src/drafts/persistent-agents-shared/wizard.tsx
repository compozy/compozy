/** S3: the kind-first wizard, on the same dialog host as today's create dialog. */
import {
  Bot,
  Check,
  ChevronDown,
  CircleAlert,
  Clock3,
  Globe,
  Infinity as PersistentIcon,
  Layers,
  type LucideIcon,
} from "lucide-react";
import type { ReactNode } from "react";

import {
  Alert,
  AlertDescription,
  Dialog,
  DialogContent,
  EntityDialogFooter,
  EntityDialogHeader,
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
  Input,
  KindIcon,
  Pill,
  RadioCard,
  Textarea,
  ToggleGroup,
  ToggleGroupItem,
  cn,
  dialogShellClass,
  providerKindIconRegistry,
} from "@compozy/ui";

import { setParam } from "@compozy/proto";

import { AgentAvatar } from "./agent-avatar";
import { TRIAGE, WORKSPACES } from "./fixtures";

export type WizardKind = "persistent" | "ephemeral";
export type WizardStep = "kind" | "identity" | "runtime" | "persona" | "where";

const STEP_LABEL: Record<WizardStep, string> = {
  kind: "Kind",
  identity: "Identity",
  runtime: "Runtime",
  persona: "Persona",
  where: "Where it runs",
};

const STEPS: Record<WizardKind, readonly WizardStep[]> = {
  persistent: ["kind", "identity", "runtime", "persona", "where"],
  ephemeral: ["kind", "identity"],
};

function StepRail({ kind, step }: { kind: WizardKind | null; step: WizardStep }) {
  const steps = STEPS[kind ?? "persistent"];
  const current = steps.indexOf(step);
  return (
    <ol className="flex min-h-0 flex-col gap-0.5 bg-sunken p-3" aria-label="Steps">
      {(kind === null ? (["kind"] as const) : steps).map((item, index) => {
        const done = index < current;
        const active = item === step;
        return (
          <li
            aria-current={active ? "step" : undefined}
            className={cn(
              "flex h-8 items-center gap-2.5 rounded-md px-2 text-small-body",
              active ? "bg-surface-2 font-medium text-fg" : done ? "text-fg-2" : "text-subtle"
            )}
            key={item}
          >
            <span
              className={cn(
                "grid size-4.5 shrink-0 place-items-center rounded-full text-micro tabular-nums",
                active
                  ? "bg-primary text-primary-foreground"
                  : done
                    ? "bg-surface-2 text-fg"
                    : "shadow-hairline-inset"
              )}
            >
              {done ? <Check aria-hidden="true" className="size-3" /> : index + 1}
            </span>
            {STEP_LABEL[item]}
          </li>
        );
      })}
    </ol>
  );
}

function KindStep({ selected, note }: { selected: WizardKind | null; note?: ReactNode }) {
  return (
    <div className="grid grid-cols-2 gap-3">
      {note ? <div className="col-span-2">{note}</div> : null}
      <RadioCard
        className="min-h-32 items-start"
        description="Stays on in a workspace, checks in on its own and keeps memory."
        icon={PersistentIcon}
        iconWellSize="lg"
        onSelect={() => setParam("state", "persistent-identity")}
        selected={selected === "persistent"}
        title="Persistent"
      />
      <RadioCard
        className="min-h-32 items-start"
        description="Runs when you start a session with it."
        icon={Clock3}
        iconWellSize="lg"
        onSelect={() => setParam("state", "ephemeral-identity")}
        selected={selected === "ephemeral"}
        title="Ephemeral"
      />
    </div>
  );
}

function AvatarPreview({ name }: { name: string }) {
  return (
    <div className="flex flex-col items-center gap-2">
      <button
        aria-label="Change avatar"
        className="grid size-20 place-items-center rounded-xl bg-sunken outline-none hover:bg-hover focus-visible:shadow-focus-ring"
        type="button"
      >
        <AgentAvatar
          appearance={{ avatar: "generated", seed: name || "agent" }}
          label={name || "New agent"}
          quietIdle
          size={48}
        />
      </button>
      <span className="text-eyebrow tracking-eyebrow text-subtle">Change</span>
    </div>
  );
}

export interface IdentityValues {
  name: string;
  description: string;
  slug: string;
  slugError?: ReactNode;
  showSlug?: boolean;
  instructions?: string;
}

function IdentityStep({ values }: { values: IdentityValues }) {
  return (
    <div className="flex flex-col gap-5">
      <div className="flex items-start gap-4">
        <AvatarPreview name={values.slug} />
        <div className="flex min-w-0 flex-1 flex-col gap-4">
          <Field>
            <FieldLabel htmlFor="wizard-name">Name</FieldLabel>
            <Input defaultValue={values.name} id="wizard-name" placeholder="Triage Lead" />
          </Field>
          <Field>
            <FieldLabel htmlFor="wizard-description">Description</FieldLabel>
            <Input
              defaultValue={values.description}
              id="wizard-description"
              placeholder="What this agent is for"
            />
          </Field>
        </div>
      </div>
      {values.instructions !== undefined ? (
        <Field>
          <FieldLabel htmlFor="wizard-instructions">Instructions</FieldLabel>
          <Textarea
            defaultValue={values.instructions}
            id="wizard-instructions"
            placeholder="Tell it what to do"
            rows={5}
          />
        </Field>
      ) : null}
      {values.showSlug ? (
        <Field data-invalid={values.slugError ? true : undefined}>
          <FieldLabel htmlFor="wizard-slug">Slug</FieldLabel>
          <Input
            aria-invalid={values.slugError ? true : undefined}
            className="font-mono"
            defaultValue={values.slug}
            id="wizard-slug"
          />
          {values.slugError ? <FieldError>{values.slugError}</FieldError> : null}
        </Field>
      ) : (
        <button
          className="inline-flex w-fit items-center gap-1 text-small-body text-muted hover:text-fg"
          onClick={() => setParam("state", "ephemeral-advanced")}
          type="button"
        >
          <ChevronDown aria-hidden="true" className="size-3.5" />
          Advanced
        </button>
      )}
    </div>
  );
}

function RuntimeStep({ extras = false }: { extras?: boolean }) {
  return (
    <div className="flex flex-col gap-5">
      <Field>
        <FieldLabel>Provider</FieldLabel>
        <ToggleGroup aria-label="Provider" defaultValue={["claude"]} multiple={false} size="sm">
          {["claude", "codex"].map(provider => (
            <ToggleGroupItem key={provider} value={provider}>
              <KindIcon
                className="size-3.5"
                kind={provider}
                registry={providerKindIconRegistry}
                size="xs"
              />
              {provider === "claude" ? "Claude" : "Codex"}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      </Field>
      <Field>
        <FieldLabel htmlFor="wizard-model">Model</FieldLabel>
        <Input defaultValue="claude-opus-5-5" id="wizard-model" className="font-mono" />
        <FieldDescription>Leave empty to use the workspace default.</FieldDescription>
      </Field>
      <Field>
        <FieldLabel>Reasoning</FieldLabel>
        <ToggleGroup aria-label="Reasoning" defaultValue={["medium"]} multiple={false} size="sm">
          {["Low", "Medium", "High"].map(level => (
            <ToggleGroupItem key={level} value={level.toLowerCase()}>
              {level}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      </Field>
      {extras ? (
        <Field>
          <FieldLabel>Tools</FieldLabel>
          <div className="flex flex-wrap gap-1.5">
            {["Files", "Terminal", "Sessions", "Browser"].map(tool => (
              <Pill key={tool} size="sm">
                {tool}
              </Pill>
            ))}
          </div>
        </Field>
      ) : null}
    </div>
  );
}

function PersonaStep() {
  return (
    <Field>
      <FieldLabel htmlFor="wizard-persona">Instructions</FieldLabel>
      <Textarea
        defaultValue={
          "You keep this project's issues moving. Flag what's blocked, name who can unblock it, and propose the next step. Stay quiet when nothing changed."
        }
        id="wizard-persona"
        rows={11}
      />
    </Field>
  );
}

export type WherePick = "global" | (typeof WORKSPACES)[number] | "not-now";

function PlaceOption({
  id,
  icon,
  title,
  description,
  pick,
}: {
  id: WherePick;
  icon: LucideIcon;
  title: string;
  description?: string;
  pick: WherePick;
}) {
  return (
    <RadioCard
      description={description}
      icon={icon}
      onSelect={() => undefined}
      selected={pick === id}
      title={title}
    />
  );
}

function WhereStep({ pick, alert }: { pick: WherePick; alert?: ReactNode }) {
  return (
    <div className="flex flex-col gap-2">
      {alert}
      <PlaceOption
        description="Runs in your User scope"
        icon={Globe}
        id="global"
        pick={pick}
        title="Global"
      />
      {WORKSPACES.map(workspace => (
        <PlaceOption
          icon={Layers}
          id={workspace}
          key={workspace}
          pick={pick}
          title={`Persistent in ${workspace}`}
        />
      ))}
      <PlaceOption
        description="Create it now and start it later"
        icon={CircleAlert}
        id="not-now"
        pick={pick}
        title="Not now"
      />
    </div>
  );
}

export function WizardAlert({
  children,
  tone = "danger",
}: {
  children: ReactNode;
  tone?: "danger" | "warning";
}) {
  return (
    <Alert className="mb-1" variant={tone}>
      <CircleAlert aria-hidden="true" className="size-4" />
      <AlertDescription>{children}</AlertDescription>
    </Alert>
  );
}

export interface WizardProps {
  kind: WizardKind | null;
  step: WizardStep;
  identity?: Partial<IdentityValues>;
  where?: WherePick;
  alert?: ReactNode;
  duplicateOf?: string;
  advanced?: boolean;
  primary?: ReactNode;
  cancel?: ReactNode;
  next?: string;
  back?: string;
  primaryDisabled?: boolean;
  hint?: ReactNode;
}

const FILLED: IdentityValues = {
  name: TRIAGE.label,
  description: TRIAGE.description,
  slug: TRIAGE.name,
};

function StepBody({ step, kind, identity, where = "compozy", alert, advanced }: WizardProps) {
  switch (step) {
    case "kind":
      return <KindStep note={alert} selected={kind} />;
    case "identity":
      return (
        <div className="flex flex-col gap-4">
          {alert}
          <IdentityStep values={{ ...FILLED, ...identity }} />
          {advanced ? <RuntimeStep extras /> : null}
        </div>
      );
    case "runtime":
      return <RuntimeStep />;
    case "persona":
      return <PersonaStep />;
    default:
      return <WhereStep alert={alert} pick={where} />;
  }
}

/** The dialog over the Agents page. */
export function AgentWizard(props: WizardProps) {
  const { kind, step, duplicateOf, primary, cancel, next, back, primaryDisabled, hint } = props;
  const title = duplicateOf ? `Duplicate ${duplicateOf}` : "New agent";
  return (
    <Dialog open>
      <DialogContent
        className={`grid-rows-[auto_minmax(0,1fr)_auto] text-fg ${dialogShellClass("lg", { fill: true })}`}
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader icon={Bot} onClose={() => setParam("state", "discard")} title={title} />
        <div className="grid min-h-0 grid-cols-[13rem_minmax(0,1fr)]">
          <StepRail kind={kind} step={step} />
          <div className="min-h-0 overflow-y-auto p-6">
            <StepBody {...props} />
          </div>
        </div>
        <EntityDialogFooter
          cancelLabel={cancel ?? (back ? "Back" : "Cancel")}
          hint={hint}
          onCancel={() => setParam("state", back ?? "discard")}
          onPrimary={() => (next ? setParam("state", next) : undefined)}
          primaryDisabled={primaryDisabled}
          primaryLabel={primary ?? "Continue"}
        />
      </DialogContent>
    </Dialog>
  );
}
