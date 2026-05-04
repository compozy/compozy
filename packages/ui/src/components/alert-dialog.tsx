"use client";

import { forwardRef, type ComponentProps, type ReactElement } from "react";

import { AlertDialog as AlertDialogPrimitive } from "@base-ui/react/alert-dialog";

import { cn } from "../lib/utils";

export const AlertDialog = AlertDialogPrimitive.Root;

export const AlertDialogPortal = AlertDialogPrimitive.Portal;

export const AlertDialogTrigger = forwardRef<HTMLButtonElement, AlertDialogPrimitive.Trigger.Props>(
  function AlertDialogTrigger(props, ref): ReactElement {
    return <AlertDialogPrimitive.Trigger ref={ref} {...props} />;
  }
);

export function AlertDialogBackdrop({
  className,
  ...props
}: AlertDialogPrimitive.Backdrop.Props): ReactElement {
  return (
    <AlertDialogPrimitive.Backdrop
      className={cn(
        "fixed inset-0 z-50 bg-black/60 backdrop-blur-[2px] transition-opacity duration-200 ease-out data-ending-style:opacity-0 data-starting-style:opacity-0",
        className
      )}
      {...props}
    />
  );
}

export function AlertDialogViewport({
  className,
  ...props
}: AlertDialogPrimitive.Viewport.Props): ReactElement {
  return (
    <AlertDialogPrimitive.Viewport
      className={cn(
        "fixed inset-0 z-50 grid items-end p-3 sm:place-items-center sm:p-6",
        className
      )}
      {...props}
    />
  );
}

export function AlertDialogContent({
  className,
  ...props
}: AlertDialogPrimitive.Popup.Props): ReactElement {
  return (
    <AlertDialogPortal>
      <AlertDialogBackdrop />
      <AlertDialogViewport>
        <AlertDialogPrimitive.Popup
          className={cn(
            "w-full max-w-xl overflow-hidden rounded-[var(--radius-xl)] border border-border-subtle bg-card text-card-foreground shadow-[var(--shadow-lg)]",
            "transition-[opacity,transform,scale] duration-200 ease-out",
            "data-ending-style:translate-y-4 data-ending-style:opacity-0 data-starting-style:translate-y-4 data-starting-style:opacity-0",
            "sm:data-ending-style:scale-[0.98] sm:data-starting-style:scale-[0.98]",
            className
          )}
          {...props}
        />
      </AlertDialogViewport>
    </AlertDialogPortal>
  );
}

export const AlertDialogPopup = AlertDialogContent;

export function AlertDialogHeader({ className, ...props }: ComponentProps<"div">): ReactElement {
  return <div className={cn("space-y-2 px-6 pt-6", className)} {...props} />;
}

export function AlertDialogFooter({ className, ...props }: ComponentProps<"div">): ReactElement {
  return (
    <div
      className={cn(
        "flex flex-col-reverse gap-2 border-t border-border-subtle bg-[color:var(--surface-inset)] px-6 py-4 sm:flex-row sm:justify-end",
        className
      )}
      {...props}
    />
  );
}

export function AlertDialogTitle({
  className,
  ...props
}: AlertDialogPrimitive.Title.Props): ReactElement {
  return (
    <AlertDialogPrimitive.Title
      className={cn("text-base font-semibold leading-6 text-foreground sm:text-lg", className)}
      {...props}
    />
  );
}

export function AlertDialogDescription({
  className,
  ...props
}: AlertDialogPrimitive.Description.Props): ReactElement {
  return (
    <AlertDialogPrimitive.Description
      className={cn("text-sm leading-6 text-muted-foreground", className)}
      {...props}
    />
  );
}

export const AlertDialogClose = forwardRef<HTMLButtonElement, AlertDialogPrimitive.Close.Props>(
  function AlertDialogClose(props, ref): ReactElement {
    return <AlertDialogPrimitive.Close ref={ref} {...props} />;
  }
);
