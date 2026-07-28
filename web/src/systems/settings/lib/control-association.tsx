import { Fragment, cloneElement, isValidElement, type ReactElement, type ReactNode } from "react";

const LABELABLE_TAGS = new Set([
  "button",
  "input",
  "meter",
  "output",
  "progress",
  "select",
  "textarea",
]);

type AssociableControlProps = {
  id?: string;
  role?: string;
  "aria-describedby"?: string;
  "aria-labelledby"?: string;
  "aria-invalid"?: boolean;
};

interface ControlAssociationOptions {
  control: ReactNode;
  controlId: string;
  labelId: string;
  descriptionId?: string;
  errorId?: string;
}

export interface AssociatedSettingsControl {
  control: ReactNode;
  labelHtmlFor?: string;
}

function mergeAttributeTokens(...values: Array<string | undefined>): string | undefined {
  const tokens = values.flatMap(value => value?.split(" ") ?? []).filter(Boolean);
  return tokens.length > 0 ? Array.from(new Set(tokens)).join(" ") : undefined;
}

function groupedControl(control: ReactNode, labelId: string, describedBy: string | undefined) {
  return (
    <div aria-describedby={describedBy} aria-labelledby={labelId} role="group">
      {control}
    </div>
  );
}

export function associateSettingsControl({
  control,
  controlId,
  labelId,
  descriptionId,
  errorId,
}: ControlAssociationOptions): AssociatedSettingsControl {
  if (!isValidElement<AssociableControlProps>(control)) {
    return { control };
  }

  const describedBy = mergeAttributeTokens(
    control.props["aria-describedby"],
    descriptionId,
    errorId
  );
  const labelledBy = mergeAttributeTokens(control.props["aria-labelledby"], labelId);
  const invalid = Boolean(errorId);

  if (control.type === Fragment) {
    return { control: groupedControl(control, labelId, describedBy) };
  }

  const isNativeRoot = typeof control.type === "string";
  if (isNativeRoot && control.type === "div") {
    return {
      control: cloneElement(control as ReactElement<AssociableControlProps>, {
        role: control.props.role ?? "group",
        "aria-labelledby": labelledBy,
        "aria-describedby": describedBy,
      }),
    };
  }

  const supportsNativeLabel = isNativeRoot && LABELABLE_TAGS.has(control.type as string);
  const id = control.props.id ?? controlId;
  return {
    control: cloneElement(control as ReactElement<AssociableControlProps>, {
      id,
      "aria-describedby": describedBy,
      "aria-labelledby": supportsNativeLabel ? control.props["aria-labelledby"] : labelledBy,
      "aria-invalid": invalid || control.props["aria-invalid"],
    }),
    labelHtmlFor: supportsNativeLabel ? id : undefined,
  };
}
