import { cva, type VariantProps } from "class-variance-authority";
import type { ButtonHTMLAttributes, ReactElement, ReactNode } from "react";

import { cn } from "../lib/utils";

export const buttonVariants = cva(
  [
    "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-[var(--radius-md)] border",
    "text-[13px] font-medium transition-[background-color,border-color,color,box-shadow,filter,transform] duration-200 ease-out",
    "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70 focus-visible:ring-offset-2 focus-visible:ring-offset-background",
    "disabled:pointer-events-none disabled:opacity-50 active:translate-y-px",
  ],
  {
    variants: {
      variant: {
        primary:
          "border-primary/50 bg-primary text-primary-foreground shadow-[0_1px_0_rgba(255,255,255,0.25)_inset,0_12px_28px_-22px_var(--primary)] hover:brightness-105",
        secondary:
          "border-border bg-secondary text-secondary-foreground shadow-[var(--shadow-xs)] hover:border-border-strong hover:bg-surface-hover hover:text-foreground",
        ghost:
          "border-transparent bg-transparent text-muted-foreground hover:bg-surface-hover hover:text-foreground",
      },
      size: {
        sm: "h-8 px-3",
        md: "h-10 px-4",
        lg: "h-11 px-5",
      },
    },
    defaultVariants: {
      variant: "primary",
      size: "md",
    },
  }
);

type ButtonBaseProps = Omit<ButtonHTMLAttributes<HTMLButtonElement>, "children"> &
  VariantProps<typeof buttonVariants> & {
    icon?: ReactNode;
    loading?: boolean;
  };

type ButtonWithChildrenProps = ButtonBaseProps & {
  children: ReactNode;
};

type IconOnlyButtonProps = ButtonBaseProps &
  (
    | {
        children?: undefined;
        "aria-label": string;
        "aria-labelledby"?: string;
      }
    | {
        children?: undefined;
        "aria-label"?: string;
        "aria-labelledby": string;
      }
  );

export type ButtonProps = ButtonWithChildrenProps | IconOnlyButtonProps;

export function Button({
  "aria-busy": ariaBusy,
  children,
  className,
  disabled,
  icon,
  loading = false,
  size,
  type = "button",
  variant,
  ...props
}: ButtonProps): ReactElement {
  const iconOnly = children === undefined || children === null;
  return (
    <button
      className={cn(
        buttonVariants({ size, variant }),
        iconOnly &&
          (size === "sm" ? "size-8 px-0" : size === "lg" ? "size-11 px-0" : "size-10 px-0"),
        className
      )}
      aria-busy={loading ? true : ariaBusy}
      disabled={loading || disabled}
      type={type}
      {...props}
    >
      {loading ? (
        <span
          aria-hidden="true"
          className="size-3.5 animate-spin rounded-full border border-current/25 border-t-current"
        />
      ) : icon ? (
        <span aria-hidden="true" className="flex size-4 items-center justify-center">
          {icon}
        </span>
      ) : null}
      {children}
    </button>
  );
}
