/** S2: pick an installed kind or a script, fill its settings, and write it into the file. */
import { Plus, X } from "@compozy/ui/icons";
import { useId, useState, type ReactNode } from "react";

import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
  Input,
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
  Spinner,
  Switch,
} from "@compozy/ui";

import type { CheckInSourceActions } from "../../hooks/use-check-ins-editor";
import {
  blankSourceDraft,
  kindOf,
  sourceChoices,
  sourceDraftFrom,
  sourceFromDraft,
  sourceParamFields,
  sourceValuePair,
  suggestSourceId,
  validateSourceDraft,
  type SourceDraft,
  type SourceDraftErrors,
  type SourceParamField,
} from "../../lib/agent-check-in-source-draft";
import { checkInSourceTitle, type CheckInSource } from "../../lib/agent-check-in-sources";
import type { AgentHeartbeatSourceCatalog } from "../../types";
import { TokenListField } from "../token-list-field";

type Catalog = AgentHeartbeatSourceCatalog;

function refLabel(ref: string, catalog: Catalog | null) {
  if (ref.startsWith("script:")) return ref.slice("script:".length);
  return kindOf(catalog, ref)?.title ?? ref.slice("kind:".length);
}

function ParamField({
  field,
  draft,
  error,
  onChange,
}: {
  field: SourceParamField;
  draft: SourceDraft;
  error?: string;
  onChange: (value: SourceDraft["params"][string]) => void;
}) {
  const id = useId();
  const value = draft.params[field.key];
  if (field.type === "list") {
    return (
      <TokenListField
        description={field.description}
        error={error}
        label={field.label}
        onChange={onChange}
        placeholder="Type and press Enter"
        testId={`source-param-${field.key}`}
        values={Array.isArray(value) ? value : []}
      />
    );
  }
  if (field.type === "boolean") {
    return (
      <Field orientation="horizontal">
        <FieldLabel htmlFor={id}>{field.label}</FieldLabel>
        <Switch checked={value === true} id={id} onCheckedChange={checked => onChange(checked)} />
      </Field>
    );
  }
  return (
    <Field data-invalid={Boolean(error) || undefined}>
      <FieldLabel htmlFor={id}>{field.label}</FieldLabel>
      {field.description ? <FieldDescription>{field.description}</FieldDescription> : null}
      <Input
        aria-invalid={Boolean(error) || undefined}
        id={id}
        onChange={event => onChange(event.target.value)}
        placeholder={field.type === "duration" ? "24h" : undefined}
        value={typeof value === "string" ? value : ""}
      />
      <FieldError>{error}</FieldError>
    </Field>
  );
}

/** A script's `with`: names and text values. Anything nested is edited in the file. */
function ValuesEditor({
  draft,
  error,
  onChange,
}: {
  draft: SourceDraft;
  error?: string;
  onChange: (pairs: SourceDraft["pairs"]) => void;
}) {
  const set = (index: number, patch: Partial<SourceDraft["pairs"][number]>) =>
    onChange(draft.pairs.map((pair, at) => (at === index ? { ...pair, ...patch } : pair)));
  return (
    <Field data-invalid={Boolean(error) || undefined}>
      <FieldLabel>Values it gets</FieldLabel>
      <FieldDescription>The script reads these as text, by name.</FieldDescription>
      {draft.pairs.map((pair, index) => (
        <div className="flex items-center gap-2" key={pair.uid}>
          <Input
            aria-label={`Name ${index + 1}`}
            onChange={event => set(index, { key: event.target.value })}
            placeholder="env"
            value={pair.key}
          />
          <Input
            aria-label={`Value ${index + 1}`}
            onChange={event => set(index, { value: event.target.value })}
            placeholder="production"
            value={pair.value}
          />
          <Button
            aria-label={`Remove value ${index + 1}`}
            onClick={() => onChange(draft.pairs.filter((_, at) => at !== index))}
            size="icon-sm"
            type="button"
            variant="quiet"
          >
            <X aria-hidden="true" />
          </Button>
        </div>
      ))}
      <div>
        <Button
          onClick={() => onChange([...draft.pairs, sourceValuePair()])}
          size="sm"
          type="button"
          variant="quiet"
        >
          <Plus aria-hidden="true" data-icon="inline-start" />
          Add a value
        </Button>
      </div>
      <FieldError>{error}</FieldError>
    </Field>
  );
}

function RefSelect({
  catalog,
  draft,
  error,
  locked,
  onPick,
}: {
  catalog: Catalog;
  draft: SourceDraft;
  error?: string;
  locked: boolean;
  onPick: (ref: string) => void;
}) {
  const { kinds, scripts } = sourceChoices(catalog);
  return (
    <Field data-invalid={Boolean(error) || undefined}>
      <FieldLabel>Reads</FieldLabel>
      <Select disabled={locked} onValueChange={value => onPick(String(value))} value={draft.ref}>
        <SelectTrigger aria-label="Reads" className="w-full">
          <SelectValue placeholder="Pick what it reads">
            {draft.ref ? refLabel(draft.ref, catalog) : null}
          </SelectValue>
        </SelectTrigger>
        <SelectContent>
          {kinds.length > 0 ? (
            <SelectGroup>
              <SelectLabel>From extensions</SelectLabel>
              {kinds.map(kind => (
                <SelectItem key={kind.kind} value={`kind:${kind.kind}`}>
                  {kind.title}
                </SelectItem>
              ))}
            </SelectGroup>
          ) : null}
          {scripts.length > 0 ? (
            <SelectGroup>
              <SelectLabel>Scripts</SelectLabel>
              {scripts.map(script => (
                <SelectItem key={script.name} value={`script:${script.name}`}>
                  {script.name}
                </SelectItem>
              ))}
            </SelectGroup>
          ) : null}
        </SelectContent>
      </Select>
      {catalog.scripts_enabled ? null : (
        <FieldDescription>
          Scripts are turned off in your settings, so none are listed.
        </FieldDescription>
      )}
      <FieldError>{error}</FieldError>
    </Field>
  );
}

function DraftFields({
  catalog,
  draft,
  errors,
  locked,
  others,
  setDraft,
}: {
  catalog: Catalog;
  draft: SourceDraft;
  errors: SourceDraftErrors;
  locked: boolean;
  others: readonly string[];
  setDraft: (draft: SourceDraft) => void;
}) {
  const nameId = useId();
  const kind = kindOf(catalog, draft.ref);
  const fields = sourceParamFields(kind?.params_schema);
  const pick = (ref: string) =>
    setDraft({
      ...blankSourceDraft(),
      ref,
      // A name the operator typed stays; an untouched one follows the pick.
      id:
        draft.id && draft.id !== suggestSourceId(draft.ref, others)
          ? draft.id
          : suggestSourceId(ref, others),
    });
  return (
    <div className="flex flex-col gap-4">
      <RefSelect catalog={catalog} draft={draft} error={errors.ref} locked={locked} onPick={pick} />
      {kind?.description ? (
        <p className="text-small-body text-muted-foreground">{kind.description}</p>
      ) : null}
      <Field data-invalid={Boolean(errors.id) || undefined}>
        <FieldLabel htmlFor={nameId}>Name</FieldLabel>
        <FieldDescription>How the file names this source, like new-issues.</FieldDescription>
        <Input
          aria-invalid={Boolean(errors.id) || undefined}
          id={nameId}
          onChange={event => setDraft({ ...draft, id: event.target.value })}
          value={draft.id}
        />
        <FieldError>{errors.id}</FieldError>
      </Field>
      {fields.map(field => (
        <ParamField
          draft={draft}
          error={errors[`param:${field.key}`]}
          field={field}
          key={field.key}
          onChange={value =>
            setDraft({ ...draft, params: { ...draft.params, [field.key]: value } })
          }
        />
      ))}
      {draft.ref.startsWith("script:") ? (
        <ValuesEditor
          draft={draft}
          error={errors.pairs}
          onChange={pairs => setDraft({ ...draft, pairs })}
        />
      ) : null}
      {Object.keys(draft.kept).length > 0 ? (
        <p className="text-small-body text-muted-foreground">
          Some of its settings can only be changed in the file. They stay as they are.
        </p>
      ) : null}
      {errors.with ? (
        <p className="text-form-hint text-destructive" role="alert">
          {errors.with}
        </p>
      ) : null}
    </div>
  );
}

function CatalogBody({
  actions,
  catalog,
  children,
}: {
  actions: CheckInSourceActions;
  catalog: Catalog | null;
  children: (catalog: Catalog) => ReactNode;
}) {
  if (actions.catalogState === "error") {
    return (
      <div className="flex items-center justify-between gap-3 text-small-body">
        <span>Couldn't load what it can read.</span>
        <Button onClick={actions.retryCatalog} size="sm" type="button" variant="neutral">
          Try again
        </Button>
      </div>
    );
  }
  if (!catalog) {
    return (
      <p className="flex items-center gap-2 text-small-body text-muted-foreground" role="status">
        <Spinner aria-hidden="true" className="size-3.5" />
        Loading sources…
      </p>
    );
  }
  if (sourceChoices(catalog).empty) {
    return (
      <p className="text-small-body text-muted-foreground">
        No sources installed. Add a script to <code className="font-mono">sources/</code> or install
        an extension.
      </p>
    );
  }
  return <>{children(catalog)}</>;
}

export interface AddCheckInSourceDialogProps {
  actions: CheckInSourceActions;
  catalog: Catalog | null;
  /** The source being edited; adding when absent. */
  source?: CheckInSource;
  /** The ids the file already uses, besides this source's. */
  others: readonly string[];
  saving: boolean;
  onClose: () => void;
}

export function AddCheckInSourceDialog({
  actions,
  catalog,
  source,
  others,
  saving,
  onClose,
}: AddCheckInSourceDialogProps) {
  const [draft, setDraft] = useState<SourceDraft | null>(null);
  const [tried, setTried] = useState(false);
  // The edited entry is read once the catalog answers, so its kind's fields can claim values.
  const current = draft ?? (source ? sourceDraftFrom(source, catalog) : blankSourceDraft());
  const errors = tried ? validateSourceDraft(current, catalog, others) : {};
  const title = source ? `Edit ${checkInSourceTitle(source, catalog)}` : "Add a source";
  const submit = () => {
    setTried(true);
    if (!catalog) return;
    if (Object.keys(validateSourceDraft(current, catalog, others)).length > 0) return;
    actions.save(sourceFromDraft(current, catalog), source?.id);
    onClose();
  };
  return (
    <Dialog onOpenChange={open => (open ? undefined : onClose())} open>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>
            It reads this at every check-in and only tells you about what's new.
          </DialogDescription>
        </DialogHeader>
        <CatalogBody actions={actions} catalog={catalog}>
          {loaded => (
            <DraftFields
              catalog={loaded}
              draft={current}
              errors={errors}
              locked={Boolean(source)}
              others={others}
              setDraft={setDraft}
            />
          )}
        </CatalogBody>
        <DialogFooter className="gap-2">
          <Button onClick={onClose} type="button" variant="neutral">
            Cancel
          </Button>
          <Button
            aria-busy={saving || undefined}
            disabled={saving || !catalog || sourceChoices(catalog).empty}
            onClick={submit}
            type="button"
          >
            {source ? "Save" : "Add source"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
