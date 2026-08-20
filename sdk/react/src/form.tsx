import type { JSONValue } from "@compozy/extension-sdk";
import { createElement } from "react";
import type { ReactElement, ReactNode } from "react";

export type FormValues = Record<string, JSONValue>;

export interface FormProps {
  children?: ReactNode;
  onSubmit?: (values: FormValues) => void | Promise<void>;
  validation?: Record<string, string>;
}

interface FormFieldBaseProps {
  id: string;
  label: string;
  placeholder?: string;
  required?: boolean;
  error?: string;
  onChange?: (value: JSONValue, eventCount: number) => void | Promise<void>;
  onBlur?: () => void | Promise<void>;
  children?: ReactNode;
}

export interface FormTextFieldProps extends FormFieldBaseProps {
  defaultValue?: string;
}

export interface FormCheckboxProps extends FormFieldBaseProps {
  defaultValue?: boolean;
}

export interface FormDropdownProps extends FormFieldBaseProps {
  defaultValue?: string;
  options?: string[];
  children?: ReactNode;
}

export interface FormDropdownItemProps {
  value: string;
  title?: string;
}

export interface FormFilePickerProps extends FormFieldBaseProps {
  defaultValue?: string[];
  directories?: boolean;
}

function FormRoot({ children, ...props }: FormProps): ReactElement {
  return createElement("view-form", props, children);
}

function field(type: string, props: FormFieldBaseProps): ReactElement {
  const { children, ...fieldProps } = props;
  return createElement("view-form-field", { ...fieldProps, type }, children as ReactNode);
}

function TextField(props: FormTextFieldProps): ReactElement {
  return field("text", props);
}

function PasswordField(props: FormTextFieldProps): ReactElement {
  return field("password", props);
}

function TextArea(props: FormTextFieldProps): ReactElement {
  return field("textarea", props);
}

function Checkbox(props: FormCheckboxProps): ReactElement {
  return field("checkbox", props);
}

function Dropdown(props: FormDropdownProps): ReactElement {
  return field("dropdown", props);
}

function DropdownItem(props: FormDropdownItemProps): ReactElement {
  return createElement("view-form-option", props);
}

function FilePicker(props: FormFilePickerProps): ReactElement {
  return field("file", props);
}

export const Form = Object.assign(FormRoot, {
  TextField,
  PasswordField,
  TextArea,
  Checkbox,
  Dropdown: Object.assign(Dropdown, { Item: DropdownItem }),
  FilePicker,
});
