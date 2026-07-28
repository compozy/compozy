"use client";

import * as React from "react";

import { cn } from "../../lib/utils";
import { Alert, AlertDescription } from "../alert";
import { Button } from "../button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "../dialog";
import { Field, FieldContent, FieldDescription, FieldLabel } from "../field";
import { Input } from "../input";

type ConfirmDialogTone = "danger" | "warning" | "accent" | "neutral";
type ConfirmDialogNoteTone = "info" | "warning" | "accent" | "neutral";
type DataAttributes = {
  [key: `data-${string}`]: string | number | boolean | undefined;
};

interface ConfirmDialogProps {
  open?: boolean;
  defaultOpen?: boolean;
  onOpenChange?: React.ComponentProps<typeof Dialog>["onOpenChange"];
  title: React.ReactNode;
  description?: React.ReactNode;
  confirmLabel: React.ReactNode;
  cancelLabel: React.ReactNode;
  tone?: ConfirmDialogTone;
  confirmTyping?: string;
  onConfirm: () => void | Promise<void>;
  isPending?: boolean;
  note?: React.ReactNode;
  noteTone?: ConfirmDialogNoteTone;
  error?: React.ReactNode;
  confirmIcon?: React.ComponentType<{ className?: string }>;
  className?: string;
  contentProps?: Omit<React.ComponentProps<typeof DialogContent>, "children"> & DataAttributes;
  titleProps?: React.ComponentProps<typeof DialogTitle> & DataAttributes;
  descriptionProps?: React.ComponentProps<typeof DialogDescription> & DataAttributes;
  cancelButtonProps?: React.ComponentProps<typeof Button> & DataAttributes;
  confirmButtonProps?: React.ComponentProps<typeof Button> & DataAttributes;
  confirmInputProps?: React.ComponentProps<typeof Input> & DataAttributes;
  noteProps?: React.ComponentProps<"div"> & DataAttributes;
  errorProps?: React.ComponentProps<"div"> & DataAttributes;
  children?: React.ReactNode;
}

const TONE_COPY: Record<ConfirmDialogTone, string> = {
  danger: "text-danger",
  warning: "text-warning",
  accent: "text-accent",
  neutral: "text-muted",
};

function ConfirmDialog({
  open,
  defaultOpen,
  onOpenChange,
  title,
  description,
  confirmLabel,
  cancelLabel,
  tone = "danger",
  confirmTyping,
  onConfirm,
  isPending = false,
  note,
  noteTone = "info",
  error,
  confirmIcon: ConfirmIcon,
  className,
  contentProps,
  titleProps,
  descriptionProps,
  cancelButtonProps,
  confirmButtonProps,
  confirmInputProps,
  noteProps,
  errorProps,
  children,
}: ConfirmDialogProps) {
  const [typedValue, setTypedValue] = React.useState("");
  const requiresTyping = typeof confirmTyping === "string" && confirmTyping.length > 0;
  const confirmBlocked = isPending || (requiresTyping && typedValue !== confirmTyping);
  const confirmVariant: React.ComponentProps<typeof Button>["variant"] =
    tone === "danger" ? "destructive" : "default";
  const handleOpenChange: React.ComponentProps<typeof Dialog>["onOpenChange"] = (
    nextOpen,
    details
  ) => {
    if (!nextOpen) {
      setTypedValue("");
    }
    onOpenChange?.(nextOpen, details);
  };

  const { className: contentClassName, ...restContentProps } = contentProps ?? {};
  const { className: titleClassName, ...restTitleProps } = titleProps ?? {};
  const { className: descriptionClassName, ...restDescriptionProps } = descriptionProps ?? {};
  const { className: noteClassName, ...restNoteProps } = noteProps ?? {};
  const { className: errorClassName, ...restErrorProps } = errorProps ?? {};
  const noteVariant = noteTone === "neutral" ? "default" : noteTone;

  return (
    <Dialog defaultOpen={defaultOpen} onOpenChange={handleOpenChange} open={open}>
      {children}
      <DialogContent
        showCloseButton={false}
        unframed
        {...restContentProps}
        className={cn("sm:max-w-md", className, contentClassName)}
      >
        <DialogHeader variant="ruled">
          <DialogTitle {...restTitleProps} className={titleClassName}>
            {title}
          </DialogTitle>
          {description ? (
            <DialogDescription
              {...restDescriptionProps}
              className={cn(TONE_COPY[tone], descriptionClassName)}
            >
              {description}
            </DialogDescription>
          ) : null}
        </DialogHeader>
        {note ? (
          <div className="px-5 pt-4">
            <Alert
              variant={noteVariant}
              role="note"
              {...restNoteProps}
              className={cn("text-xs", noteClassName)}
            >
              <AlertDescription>{note}</AlertDescription>
            </Alert>
          </div>
        ) : null}
        {requiresTyping ? (
          <div className="px-5 py-4">
            <Field>
              <FieldContent>
                <FieldLabel htmlFor={confirmInputProps?.id ?? "confirm-dialog-typing"}>
                  Type to confirm
                </FieldLabel>
                <FieldDescription>
                  Enter <span className="font-mono">{confirmTyping}</span> to enable this action.
                </FieldDescription>
              </FieldContent>
              <Input
                autoComplete="off"
                {...confirmInputProps}
                id={confirmInputProps?.id ?? "confirm-dialog-typing"}
                onChange={event => {
                  setTypedValue(event.target.value);
                  confirmInputProps?.onChange?.(event);
                }}
                value={typedValue}
              />
            </Field>
          </div>
        ) : null}
        {error ? (
          <div className="border-t border-line px-5 py-3">
            <Alert variant="danger" {...restErrorProps} className={cn("text-xs", errorClassName)}>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          </div>
        ) : null}
        <DialogFooter variant="ruled">
          <DialogClose
            render={<Button size="sm" type="button" variant="ghost" {...cancelButtonProps} />}
          >
            {cancelLabel}
          </DialogClose>
          <Button
            disabled={confirmBlocked}
            size="sm"
            type="button"
            variant={confirmVariant}
            {...confirmButtonProps}
            onClick={event => {
              confirmButtonProps?.onClick?.(event);
              if (event.defaultPrevented) return;
              void onConfirm();
            }}
          >
            {ConfirmIcon ? <ConfirmIcon className="size-3" /> : null}
            {confirmLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export { ConfirmDialog };
export type { ConfirmDialogNoteTone, ConfirmDialogProps, ConfirmDialogTone };
