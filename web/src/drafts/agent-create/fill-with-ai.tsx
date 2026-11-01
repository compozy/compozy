/** Fill with AI: describe the job, pick a model, and it writes the instructions. */
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

import { RuntimeSelector } from "@/systems/runtime/components/runtime-selector";

import { useFillWithAi, type FillPreset } from "./hooks/use-fill-with-ai";
import { MODELS, modelLabel, providersFor, type Draft } from "./model";

export type { FillPreset };

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

export function FillWithAi({
  value,
  onChange,
  runtime,
  signedOut,
  preset,
}: {
  value: string;
  onChange: (next: string) => void;
  runtime: Draft["runtime"];
  signedOut?: string;
  preset?: FillPreset;
}) {
  const fill = useFillWithAi({ value, onChange, runtime, preset });
  const { status, brief, model } = fill;
  const generating = status === "generating";
  const applied = status === "applied";
  const failed = status === "failed";

  return (
    <Popover onOpenChange={fill.onOpenChange} open={fill.open}>
      <PopoverTrigger render={<Button size="xs" type="button" variant="secondary" />}>
        {generating ? <Spinner aria-hidden="true" /> : <Sparkles aria-hidden="true" />}
        Fill with AI
      </PopoverTrigger>
      <PopoverContent align="end" className="flex w-84 flex-col gap-3 p-3.5" sideOffset={8}>
        <PopoverTitle className="flex items-center gap-2 text-small-body font-medium text-fg">
          <Sparkles aria-hidden="true" className="size-3.5 text-muted-foreground" />
          Fill with AI
        </PopoverTitle>
        {applied ? (
          <Message applied model={modelLabel(model)} />
        ) : (
          <>
            {failed ? <Message applied={false} model={modelLabel(model)} /> : null}
            <Textarea
              aria-label="What should this agent do?"
              autoFocus
              disabled={generating}
              onChange={event => fill.setBrief(event.target.value)}
              placeholder="What should this agent do?"
              rows={3}
              value={brief}
            />
            {value.trim() !== "" && !generating ? (
              <p className="text-form-hint text-subtle">
                This replaces what's written now. You can undo it.
              </p>
            ) : null}
          </>
        )}
        {applied ? (
          <div className="flex justify-end gap-2">
            <Button onClick={fill.undo} size="sm" type="button" variant="ghost">
              Undo
            </Button>
            <Button onClick={fill.finish} size="sm" type="button">
              Done
            </Button>
          </div>
        ) : (
          <div className="flex items-center justify-between gap-2">
            <RuntimeSelector
              disabled={generating}
              models={MODELS}
              onChange={next => fill.setModel(next)}
              providers={providersFor(signedOut)}
              value={model}
              variant="composer"
            />
            {generating ? (
              <Button onClick={fill.stop} size="sm" type="button" variant="secondary">
                Stop
              </Button>
            ) : (
              <Button disabled={brief.trim() === ""} onClick={fill.start} size="sm" type="button">
                {failed ? "Try again" : "Write it"}
              </Button>
            )}
          </div>
        )}
      </PopoverContent>
    </Popover>
  );
}
