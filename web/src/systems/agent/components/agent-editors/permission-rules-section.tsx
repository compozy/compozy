/** S14: a short list of exceptions, closed until someone needs one. */
import { Plus, X } from "@compozy/ui/icons";
import { useId } from "react";

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

import {
  PERMISSION_RULE_KINDS,
  PERMISSION_RULE_LIMIT,
  permissionRuleKindLabel,
  type PermissionRuleDecision,
  type PermissionRuleDraft,
} from "../../lib/agent-permission-rules";

function kindOptions(current: string) {
  return PERMISSION_RULE_KINDS.some(kind => kind.value === current)
    ? PERMISSION_RULE_KINDS
    : [...PERMISSION_RULE_KINDS, { value: current, label: current }];
}

function RuleRow({
  rule,
  invalid,
  describedBy,
  disabled,
  onChange,
  onRemove,
}: {
  rule: PermissionRuleDraft;
  invalid: boolean;
  /** Id of the message that explains why this row can't be saved. */
  describedBy?: string;
  disabled: boolean;
  onChange: (rule: PermissionRuleDraft) => void;
  onRemove: () => void;
}) {
  return (
    <li
      className="grid grid-cols-[9rem_minmax(0,1fr)_7rem_auto] items-center gap-2"
      data-testid="agent-permission-rule"
    >
      <Select
        disabled={disabled}
        onValueChange={value => onChange({ ...rule, kind: String(value) })}
        value={rule.kind}
      >
        <SelectTrigger aria-label="Action" className="w-full" size="sm">
          <SelectValue>{permissionRuleKindLabel(rule.kind)}</SelectValue>
        </SelectTrigger>
        <SelectContent>
          {kindOptions(rule.kind).map(kind => (
            <SelectItem key={kind.value} value={kind.value}>
              {kind.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Input
        aria-describedby={invalid ? describedBy : undefined}
        aria-invalid={invalid || undefined}
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
          onChange({ ...rule, decision: value === "deny" ? "deny" : "allow" })
        }
        value={rule.decision}
      >
        <SelectTrigger aria-label="Decision" className="w-full" size="sm">
          <SelectValue>{rule.decision === "allow" ? "Allow" : "Never"}</SelectValue>
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={"allow" satisfies PermissionRuleDecision}>Allow</SelectItem>
          <SelectItem value={"deny" satisfies PermissionRuleDecision}>Never</SelectItem>
        </SelectContent>
      </Select>
      {disabled ? (
        <span aria-hidden="true" className="size-7" />
      ) : (
        <Button
          aria-label="Remove exception"
          onClick={onRemove}
          size="icon-sm"
          type="button"
          variant="quiet"
        >
          <X aria-hidden="true" />
        </Button>
      )}
    </li>
  );
}

export function PermissionRulesSection({
  rules,
  invalid,
  readOnly,
  onChange,
}: {
  rules: readonly PermissionRuleDraft[];
  /** The first rule that can't be saved, and why. */
  invalid: { id: string; message: string } | null;
  readOnly: boolean;
  onChange: (rules: PermissionRuleDraft[]) => void;
}) {
  const messageId = useId();
  const atLimit = rules.length >= PERMISSION_RULE_LIMIT;
  const add = () =>
    onChange([...rules, { id: crypto.randomUUID(), kind: "edit", path: "", decision: "allow" }]);
  return (
    <FormSection
      data-testid="agent-permission-rules"
      help="Only you can change these. Its child sessions follow their own."
      title="Exceptions"
    >
      {rules.length > 0 ? (
        <ul aria-label="Exceptions" className="flex flex-col gap-2">
          {rules.map(rule => (
            <RuleRow
              disabled={readOnly}
              describedBy={messageId}
              invalid={rule.id === invalid?.id}
              key={rule.id}
              onChange={next => onChange(rules.map(item => (item.id === rule.id ? next : item)))}
              onRemove={() => onChange(rules.filter(item => item.id !== rule.id))}
              rule={rule}
            />
          ))}
        </ul>
      ) : null}
      {invalid ? (
        <p className="text-small-body text-destructive" id={messageId} role="alert">
          {invalid.message}
        </p>
      ) : null}
      {readOnly ? null : atLimit ? (
        <p className="text-small-body text-muted-foreground">
          That's the most it can have: {PERMISSION_RULE_LIMIT} exceptions.
        </p>
      ) : (
        <div>
          <Button onClick={add} size="sm" type="button" variant="secondary">
            <Plus aria-hidden="true" />
            Add an exception
          </Button>
        </div>
      )}
      {rules.length > 0 ? (
        <Alert data-testid="agent-permission-rules-coverage">
          <AlertDescription>
            Exceptions apply when the provider asks first. They can't see inside commands, and
            anything the provider does without asking isn't covered.
          </AlertDescription>
        </Alert>
      ) : null}
    </FormSection>
  );
}
