/** S14: a short list of exceptions, closed until someone needs one. */
import { Plus, X } from "@compozy/ui/icons";

import {
  Alert,
  AlertDescription,
  Button,
  FormSection,
  Input,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@compozy/ui";

import { RULE_KINDS, type PermissionRule, type RuleKind } from "./levels";

function RuleRow({
  rule,
  invalid,
  disabled,
  onChange,
  onRemove,
}: {
  rule: PermissionRule;
  invalid?: boolean;
  disabled?: boolean;
  onChange: (rule: PermissionRule) => void;
  onRemove: () => void;
}) {
  const kindLabel = RULE_KINDS.find(kind => kind.value === rule.kind)?.label;
  return (
    <li className="grid grid-cols-[9rem_minmax(0,1fr)_7rem_auto] items-center gap-2">
      <Select
        disabled={disabled}
        onValueChange={value => onChange({ ...rule, kind: value as RuleKind })}
        value={rule.kind}
      >
        <SelectTrigger aria-label="Action" className="w-full" size="sm">
          <SelectValue>{kindLabel}</SelectValue>
        </SelectTrigger>
        <SelectContent>
          {RULE_KINDS.map(kind => (
            <SelectItem key={kind.value} value={kind.value}>
              {kind.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Input
        aria-invalid={invalid}
        aria-label="Folder or file"
        className="font-mono"
        disabled={disabled}
        onChange={event => onChange({ ...rule, path: event.target.value })}
        placeholder="Anywhere"
        value={rule.path}
      />
      <Select
        disabled={disabled}
        onValueChange={value =>
          onChange({ ...rule, decision: value as PermissionRule["decision"] })
        }
        value={rule.decision}
      >
        <SelectTrigger aria-label="Decision" className="w-full" size="sm">
          <SelectValue>{rule.decision === "allow" ? "Allow" : "Never"}</SelectValue>
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="allow">Allow</SelectItem>
          <SelectItem value="deny">Never</SelectItem>
        </SelectContent>
      </Select>
      <Button
        aria-label="Remove exception"
        disabled={disabled}
        onClick={onRemove}
        size="icon-sm"
        type="button"
        variant="quiet"
      >
        <X aria-hidden="true" />
      </Button>
    </li>
  );
}

export function RulesSection({
  rules,
  invalidId,
  readOnly,
  onChange,
}: {
  rules: readonly PermissionRule[];
  invalidId?: string;
  readOnly?: boolean;
  onChange: (rules: readonly PermissionRule[]) => void;
}) {
  const add = () =>
    onChange([...rules, { id: `rule-${rules.length}`, kind: "edit", path: "", decision: "allow" }]);
  return (
    <FormSection
      help="Only you can change these. Its child sessions follow their own."
      title="Exceptions"
    >
      {rules.length > 0 ? (
        <ul className="flex flex-col gap-2">
          {rules.map(rule => (
            <RuleRow
              disabled={readOnly}
              invalid={rule.id === invalidId}
              key={rule.id}
              onChange={next =>
                onChange(rules.map(item => (item.id === rule.id ? next : item)))
              }
              onRemove={() => onChange(rules.filter(item => item.id !== rule.id))}
              rule={rule}
            />
          ))}
        </ul>
      ) : null}
      {invalidId ? (
        <p className="text-small-body text-destructive">
          Use a path inside the project, such as docs/** or src/app.
        </p>
      ) : null}
      {readOnly ? null : (
        <div>
          <Button onClick={add} size="sm" type="button" variant="secondary">
            <Plus aria-hidden="true" />
            Add an exception
          </Button>
        </div>
      )}
      {rules.length > 0 ? (
        <Alert>
          <AlertDescription>
            Exceptions apply when the provider asks first. They can't see inside commands, and
            anything the provider does without asking isn't covered.
          </AlertDescription>
        </Alert>
      ) : null}
    </FormSection>
  );
}
