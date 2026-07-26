import { AlertCircleIcon, CheckIcon, XIcon } from "lucide-react";
import type * as React from "react";

import { cn } from "../../lib/utils";
import { Button } from "../button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "../dropdown-menu";
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
  InputGroupText,
} from "../input-group";
import { Tooltip, TooltipContent, TooltipTrigger } from "../tooltip";
import type { FilterFieldConfig, FilterOperator } from "./filter-types";
import { useFilterContext, type FilterI18nConfig } from "./hooks/use-filter-context";
import { useFilterInput } from "./hooks/use-filter-input";

function FilterInput<T = unknown>({
  field,
  focusOnMount,
  onBlur,
  onKeyDown,
  className,
  ...props
}: React.InputHTMLAttributes<HTMLInputElement> & {
  className?: string;
  field?: FilterFieldConfig<T>;
  focusOnMount?: boolean;
}) {
  const {
    context,
    focusInputOnMount,
    handleKeyDown,
    isValid,
    validateFilterInputOnBlur,
    validationMessage,
  } = useFilterInput({
    field,
    focusOnMount,
    onBlur,
    onKeyDown,
    pattern: props.pattern,
  });

  return (
    <InputGroup
      className={cn(
        "w-36",
        context.size === "sm" && "h-7!",
        context.size === "default" && "h-8!",
        context.size === "lg" && "h-9!",
        className
      )}
    >
      {field?.prefix ? (
        <InputGroupAddon>
          <InputGroupText>{field.prefix}</InputGroupText>
        </InputGroupAddon>
      ) : null}
      <InputGroupInput
        ref={focusInputOnMount}
        aria-invalid={!isValid}
        aria-describedby={
          !isValid && validationMessage ? `${field?.key || "input"}-error` : undefined
        }
        onBlur={validateFilterInputOnBlur}
        onKeyDown={handleKeyDown}
        className={cn(
          context.size === "sm" && "h-7! text-form-label",
          context.size === "default" && "h-8!",
          context.size === "lg" && "h-9!"
        )}
        {...props}
      />
      {!isValid && validationMessage ? (
        <InputGroupAddon align="inline-end">
          <Tooltip>
            <TooltipTrigger render={<InputGroupButton size="icon-xs" />}>
              <AlertCircleIcon className="size-3 text-destructive" />
            </TooltipTrigger>
            <TooltipContent>
              <p id={`${field?.key || "input"}-error`}>{validationMessage}</p>
            </TooltipContent>
          </Tooltip>
        </InputGroupAddon>
      ) : null}
      {field?.suffix ? (
        <InputGroupAddon align="inline-end">
          <InputGroupText>{field.suffix}</InputGroupText>
        </InputGroupAddon>
      ) : null}
    </InputGroup>
  );
}

interface FilterRemoveButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  icon?: React.ReactNode;
}

function FilterRemoveButton({ className, icon = <XIcon />, ...props }: FilterRemoveButtonProps) {
  const context = useFilterContext();

  return (
    <Button
      variant="outline"
      size={context.size === "sm" ? "icon-sm" : context.size === "lg" ? "icon-lg" : "icon"}
      className={className}
      {...props}
    >
      {icon}
    </Button>
  );
}

const createOperatorsFromI18n = (i18n: FilterI18nConfig): Record<string, FilterOperator[]> => ({
  select: [
    { value: "is", label: i18n.operators.is },
    { value: "is_not", label: i18n.operators.isNot },
    { value: "empty", label: i18n.operators.empty },
    { value: "not_empty", label: i18n.operators.notEmpty },
  ],
  multiselect: [
    { value: "is_any_of", label: i18n.operators.isAnyOf },
    { value: "is_not_any_of", label: i18n.operators.isNotAnyOf },
    { value: "includes_all", label: i18n.operators.includesAll },
    { value: "excludes_all", label: i18n.operators.excludesAll },
    { value: "empty", label: i18n.operators.empty },
    { value: "not_empty", label: i18n.operators.notEmpty },
  ],
  text: [
    { value: "contains", label: i18n.operators.contains },
    { value: "not_contains", label: i18n.operators.notContains },
    { value: "starts_with", label: i18n.operators.startsWith },
    { value: "ends_with", label: i18n.operators.endsWith },
    { value: "is", label: i18n.operators.isExactly },
    { value: "empty", label: i18n.operators.empty },
    { value: "not_empty", label: i18n.operators.notEmpty },
  ],
  custom: [
    { value: "is", label: i18n.operators.is },
    { value: "after", label: i18n.operators.after },
    { value: "before", label: i18n.operators.before },
    { value: "between", label: i18n.operators.between },
    { value: "empty", label: i18n.operators.empty },
    { value: "not_empty", label: i18n.operators.notEmpty },
  ],
});

function getOperatorsForField<T>(
  field: FilterFieldConfig<T>,
  values: T[],
  i18n: FilterI18nConfig
): FilterOperator[] {
  if (field.operators) return field.operators;

  const operators = createOperatorsFromI18n(i18n);
  let fieldType = field.type || "select";
  if (fieldType === "select" && values.length > 1) fieldType = "multiselect";
  if (fieldType === "multiselect") return operators.multiselect;
  return operators[fieldType] || operators.select;
}

interface FilterOperatorDropdownProps<T = unknown> {
  field: FilterFieldConfig<T>;
  operator: string;
  values: T[];
  onChange: (operator: string) => void;
}

function FilterOperatorDropdown<T = unknown>({
  field,
  operator,
  values,
  onChange,
}: FilterOperatorDropdownProps<T>) {
  const context = useFilterContext();
  const operators = getOperatorsForField(field, values, context.i18n);
  const operatorLabel =
    operators.find(candidate => candidate.value === operator)?.label ||
    context.i18n.helpers.formatOperator(operator);

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            variant="outline"
            size={context.size}
            className="text-muted-foreground hover:text-foreground"
          >
            {operatorLabel}
          </Button>
        }
      />
      <DropdownMenuContent align="start" className="w-fit min-w-fit">
        {operators.map(candidate => (
          <DropdownMenuItem
            key={candidate.value}
            onClick={() => onChange(candidate.value)}
            className="flex items-center justify-between data-highlighted:bg-accent data-highlighted:text-accent-foreground"
          >
            <span>{candidate.label}</span>
            <CheckIcon
              className={cn(
                "ms-auto text-primary",
                candidate.value === operator ? "opacity-100" : "opacity-0"
              )}
            />
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export { FilterInput, FilterOperatorDropdown, FilterRemoveButton };
