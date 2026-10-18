import { Button as ButtonPrimitive } from "@base-ui/react/button";
import type { VariantProps } from "class-variance-authority";
import type * as React from "react";

import { cn } from "../lib/utils";
import { buttonVariants } from "./button-variants";
import { Kbd } from "./kbd";

type ButtonProps = ButtonPrimitive.Props &
  VariantProps<typeof buttonVariants> & {
    /**
     * Trailing key hint (e.g. `↵` on "Allow once"). Decorative: it renders in the
     * button's own ink and stays out of the accessible name; pair it with
     * `aria-keyshortcuts` when the chord is live.
     */
    kbd?: React.ReactNode;
  };

function Button({ className, variant = "default", size = "default", kbd, ...props }: ButtonProps) {
  return (
    <ButtonPrimitive
      data-slot="button"
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
      {...(kbd == null
        ? null
        : {
            children: (
              <>
                {props.children as React.ReactNode}
                <Kbd aria-hidden="true">{kbd}</Kbd>
              </>
            ),
          })}
    />
  );
}

export { Button };
