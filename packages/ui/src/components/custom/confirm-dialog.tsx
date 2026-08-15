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
import { Eyebrow } from "./eyebrow";

type ConfirmDialogTone = "danger" | "warning" | "accent" | "neutral";
type ConfirmDialogIconTone = "accent" | "neutral" | "danger";
type ConfirmDialogNoteTone = "info" | "warning" | "accent" | "neutral";
type ConfirmDialogIcon = React.ComponentType<{ className?: string }>;
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
  confirmIcon?: ConfirmDialogIcon;
  /** 36px head well before the title block. Tone chrome comes from `iconTone`. */
  icon?: ConfirmDialogIcon;
  /** Paints the icon well only. Defaults from `tone` (`warning` → `neutral`). */
  iconTone?: ConfirmDialogIconTone;
  /** Header sibling above `DialogTitle` — never nested inside the accessible name. */
  eyebrow?: React.ReactNode;
  /** Left-aligned footer slot (icon + mono micro) sharing the ruled row. */
  footNote?: React.ReactNode;
  className?: string;
  contentProps?: Omit<React.ComponentProps<typeof DialogContent>, "children"> & DataAttributes;
  titleProps?: React.ComponentProps<typeof DialogTitle> & DataAttributes;
  descriptionProps?: React.ComponentProps<typeof DialogDescription> & DataAttributes;
  cancelButtonProps?: React.ComponentProps<typeof Button> & DataAttributes;
  confirmButtonProps?: React.ComponentProps<typeof Button> & DataAttributes;
  confirmInputProps?: React.ComponentProps<typeof Input> & DataAttributes;
  noteProps?: React.ComponentProps<"div"> & DataAttributes;
  errorProps?: React.ComponentProps<"div"> & DataAttributes;
  /**
   * Extra content inside the dialog body, between the note and the footer — a
   * mode choice, a payload field, a machine trail. `children` is the Dialog's
   * trigger slot, so anything meant to appear *inside* the dialog belongs here.
   */
  body?: React.ReactNode;
  bodyProps?: React.ComponentProps<"div"> & DataAttributes;
  children?: React.ReactNode;
}

const TONE_EYEBROW: Record<ConfirmDialogTone, string> = {
  danger: "text-danger",
  warning: "text-warning",
  accent: "text-accent-strong",
  neutral: "text-muted",
};

const ICON_WELL_TONE: Record<ConfirmDialogIconTone, string> = {
  accent: "bg-accent-tint text-accent-strong ring-1 ring-accent-dim ring-inset",
  neutral: "bg-canvas-tint text-muted ring-1 ring-line ring-inset",
  danger: "bg-danger-tint text-danger ring-1 ring-danger/24 ring-inset",
};

function resolveIconTone(
  iconTone: ConfirmDialogIconTone | undefined,
  tone: ConfirmDialogTone
): ConfirmDialogIconTone {
  if (iconTone) return iconTone;
  if (tone === "danger") return "danger";
  if (tone === "accent") return "accent";
  return "neutral";
}

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
  icon: Icon,
  iconTone,
  eyebrow,
  footNote,
  className,
  contentProps,
  titleProps,
  descriptionProps,
  cancelButtonProps,
  confirmButtonProps,
  confirmInputProps,
  noteProps,
  errorProps,
  body,
  bodyProps,
  children,
}: ConfirmDialogProps) {
  const [typedValue, setTypedValue] = React.useState("");
  const requiresTyping = typeof confirmTyping === "string" && confirmTyping.length > 0;
  const confirmBlocked = isPending || (requiresTyping && typedValue !== confirmTyping);
  const confirmVariant: React.ComponentProps<typeof Button>["variant"] =
    tone === "danger" || tone === "warning" ? "destructive" : "default";
  const resolvedIconTone = resolveIconTone(iconTone, tone);
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
  const { className: bodyClassName, ...restBodyProps } = bodyProps ?? {};
  const noteVariant = noteTone === "neutral" ? "default" : noteTone;

  const titleBlock = (
    <>
      {eyebrow ? <Eyebrow className={TONE_EYEBROW[tone]}>{eyebrow}</Eyebrow> : null}
      <DialogTitle {...restTitleProps} className={cn(eyebrow ? "mt-1" : undefined, titleClassName)}>
        {title}
      </DialogTitle>
      {description ? (
        <DialogDescription
          {...restDescriptionProps}
          className={cn(eyebrow || Icon ? "mt-1" : undefined, descriptionClassName)}
        >
          {description}
        </DialogDescription>
      ) : null}
    </>
  );

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
          {Icon ? (
            <div className="flex items-start gap-3">
              <div
                aria-hidden="true"
                data-icon-tone={resolvedIconTone}
                data-slot="confirm-dialog-icon"
                className={cn(
                  "flex size-9 shrink-0 items-center justify-center rounded-icon-well",
                  ICON_WELL_TONE[resolvedIconTone]
                )}
              >
                <Icon className="size-4" />
              </div>
              <div className="min-w-0 flex-1">{titleBlock}</div>
            </div>
          ) : (
            titleBlock
          )}
        </DialogHeader>
        {note || body ? (
          <div
            data-slot="confirm-dialog-stack"
            className="flex min-w-0 flex-col gap-4 px-5 pt-4 pb-5"
          >
            {note ? (
              <Alert
                variant={noteVariant}
                role="note"
                {...restNoteProps}
                className={cn("text-xs", noteClassName)}
              >
                <AlertDescription>{note}</AlertDescription>
              </Alert>
            ) : null}
            {body ? (
              <div
                data-slot="confirm-dialog-body"
                {...restBodyProps}
                className={cn("flex min-w-0 flex-col gap-3", bodyClassName)}
              >
                {body}
              </div>
            ) : null}
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
        <DialogFooter
          variant="ruled"
          className={cn(
            footNote && "sm:justify-between max-[760px]:flex-col max-[760px]:items-stretch"
          )}
        >
          {footNote ? (
            <div
              data-slot="confirm-dialog-footnote"
              className="flex min-w-0 flex-1 items-center gap-1.5 font-mono text-mono-id text-muted [&_svg]:size-3.5 [&_svg]:shrink-0 [&_svg]:text-faint"
            >
              {footNote}
            </div>
          ) : null}
          <div
            className="flex shrink-0 items-center justify-end gap-2"
            data-slot="confirm-dialog-actions"
          >
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
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export { ConfirmDialog };
export type { ConfirmDialogIconTone, ConfirmDialogNoteTone, ConfirmDialogProps, ConfirmDialogTone };
