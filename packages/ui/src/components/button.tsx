import { cva, type VariantProps } from "class-variance-authority";
import type { ButtonHTMLAttributes, ReactElement, ReactNode } from "react";

import { cn } from "../lib/utils";

export const buttonVariants = cva(
  [
    "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-[calc(var(--radius)-2px)] border",
    "text-[13px] font-medium transition-colors duration-150 ease-out focus-visible:outline-none",
    "focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:ring-offset-0",
    "disabled:pointer-events-none disabled:opacity-50 active:translate-y-px",
  ],
  {
    variants: {
      variant: {
        primary: "border-transparent bg-primary text-primary-foreground hover:brightness-95",
        secondary:
          "border-border bg-card text-card-foreground hover:border-primary/40 hover:text-primary",
        ghost:
          "border-transparent bg-transparent text-muted-foreground hover:bg-accent hover:text-foreground",
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

export interface ButtonProps
  extends ButtonHTMLAttributes<HTMLButtonElement>, VariantProps<typeof buttonVariants> {
  icon?: ReactNode;
}

export function Button({
  children,
  className,
  icon,
  size,
  type = "button",
  variant,
  ...props
}: ButtonProps): ReactElement {
  return (
    <button className={cn(buttonVariants({ className, size, variant }))} type={type} {...props}>
      {icon ? (
        <span aria-hidden="true" className="flex size-4 items-center justify-center">
          {icon}
        </span>
      ) : null}
      {children}
    </button>
  );
}
