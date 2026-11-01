import { Check, Sparkles, TriangleAlert } from "@compozy/ui/icons";

import {
  Button,
  Popover,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
  Spinner,
  Textarea,
} from "@compozy/ui";

import {
  RuntimeSelector,
  type RuntimeModelOption,
  type RuntimeProviderOption,
  type RuntimeSelectorValue,
} from "@/systems/runtime";

import { useAgentFillWithAi } from "../hooks/use-agent-fill-with-ai";

function Message({ applied, model }: { applied: boolean; model: string }) {
  const Icon = applied ? Check : TriangleAlert;
  return (
    <div className="flex items-start gap-2.5" role="status">
      <Icon
        aria-hidden="true"
        className={applied ? "mt-0.5 size-4 text-success" : "mt-0.5 size-4 text-destructive"}
      />
      <p className="min-w-0 text-small-body text-muted-foreground">
        <span className="block text-fg">
          {applied ? "Instructions added" : `${model} couldn't write them`}
        </span>
        {applied
          ? "Read them over and change anything you like."
          : "Nothing was changed. Try again or pick another model."}
      </p>
    </div>
  );
}

type FillModel = ReturnType<typeof useAgentFillWithAi>;

function FillBrief({
  fill,
  value,
  modelName,
}: {
  fill: FillModel;
  value: string;
  modelName: string;
}) {
  const generating = fill.status === "generating";
  return (
    <>
      {fill.status === "failed" ? <Message applied={false} model={modelName} /> : null}
      <Textarea
        aria-label="What should this agent do?"
        autoFocus
        disabled={generating}
        onChange={event => fill.setBrief(event.target.value)}
        placeholder="What should this agent do?"
        rows={3}
        value={fill.brief}
      />
      {value.trim() !== "" && !generating ? (
        <p className="text-form-hint text-subtle">
          This replaces what's written now. You can undo it.
        </p>
      ) : null}
    </>
  );
}

function FillBriefActions({
  fill,
  providers,
  models,
}: {
  fill: FillModel;
  providers: RuntimeProviderOption[];
  models: RuntimeModelOption[];
}) {
  const { status, brief, model } = fill;
  const generating = status === "generating";
  return (
    <div className="flex items-center justify-between gap-2">
      <RuntimeSelector
        disabled={generating}
        models={models}
        onChange={next => fill.setModel(next)}
        providers={providers}
        value={model}
        variant="composer"
      />
      {generating ? (
        <Button onClick={fill.stop} size="sm" type="button" variant="secondary">
          Stop
        </Button>
      ) : (
        <Button
          disabled={brief.trim() === "" || !model.provider}
          onClick={() => void fill.start()}
          size="sm"
          type="button"
        >
          {status === "failed" ? "Try again" : "Write it"}
        </Button>
      )}
    </div>
  );
}

function FillApplied({ fill, modelName }: { fill: FillModel; modelName: string }) {
  return (
    <>
      <Message applied model={modelName} />
      <div className="flex justify-end gap-2">
        <Button onClick={fill.undo} size="sm" type="button" variant="ghost">
          Undo
        </Button>
        <Button onClick={fill.finish} size="sm" type="button">
          Done
        </Button>
      </div>
    </>
  );
}

/** Fill with AI: describe the job, pick a model, and it writes the instructions. */
export function AgentFillWithAi({
  value,
  onChange,
  runtime,
  providers,
  models,
}: {
  value: string;
  onChange: (next: string) => void;
  runtime: RuntimeSelectorValue;
  providers: RuntimeProviderOption[];
  models: RuntimeModelOption[];
}) {
  const fill = useAgentFillWithAi({ value, onChange, runtime });
  const { status, model } = fill;
  const modelName =
    models.find(option => option.id === model.model)?.name ?? (model.model || "This model");

  return (
    <Popover onOpenChange={fill.onOpenChange} open={fill.open}>
      <PopoverTrigger
        render={
          <Button data-testid="agent-fill-with-ai" size="xs" type="button" variant="secondary" />
        }
      >
        {status === "generating" ? <Spinner aria-hidden="true" /> : <Sparkles aria-hidden="true" />}
        Fill with AI
      </PopoverTrigger>
      <PopoverContent align="end" className="flex w-84 flex-col gap-3 p-3.5" sideOffset={8}>
        <PopoverTitle className="flex items-center gap-2 text-small-body font-medium text-fg">
          <Sparkles aria-hidden="true" className="size-3.5 text-muted-foreground" />
          Fill with AI
        </PopoverTitle>
        {status === "applied" ? (
          <FillApplied fill={fill} modelName={modelName} />
        ) : (
          <>
            <FillBrief fill={fill} modelName={modelName} value={value} />
            <FillBriefActions fill={fill} models={models} providers={providers} />
          </>
        )}
      </PopoverContent>
    </Popover>
  );
}
