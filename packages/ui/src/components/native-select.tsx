import * as React from "react";
import { ChevronDownIcon } from "lucide-react";

import { cn } from "../lib/utils";

type NativeSelectProps = Omit<React.ComponentProps<"select">, "size"> & {
  size?: "sm" | "default";
};

function NativeSelect({ className, size = "default", ...props }: NativeSelectProps) {
  return (
    <div
      className={cn(
        "group/native-select relative w-fit has-[select:disabled]:opacity-50",
        className
      )}
      data-slot="native-select-wrapper"
      data-size={size}
    >
      <select
        data-slot="native-select"
        data-size={size}
        className="h-input w-full min-w-0 appearance-none rounded-md border border-line bg-elevated py-0 pr-9 pl-3 text-small-body text-fg transition-colors outline-none select-none selection:bg-accent-tint-strong selection:text-fg placeholder:text-subtle focus-visible:outline-none focus-visible:border-line-strong focus-visible:shadow-focus-ring disabled:pointer-events-none disabled:cursor-not-allowed disabled:border-line-soft disabled:bg-canvas disabled:text-disabled disabled:opacity-100 aria-invalid:border-danger aria-invalid:shadow-none data-[size=sm]:h-control-compact data-[size=sm]:rounded-md data-[size=sm]:pr-8 data-[size=sm]:pl-2.5"
        {...props}
      />
      <ChevronDownIcon
        className="pointer-events-none absolute top-1/2 right-2.5 size-4 -translate-y-1/2 text-subtle select-none"
        aria-hidden="true"
        data-slot="native-select-icon"
      />
    </div>
  );
}

function NativeSelectOption({ ...props }: React.ComponentProps<"option">) {
  return <option data-slot="native-select-option" {...props} />;
}

function NativeSelectOptGroup({ className, ...props }: React.ComponentProps<"optgroup">) {
  return <optgroup data-slot="native-select-optgroup" className={cn(className)} {...props} />;
}

export { NativeSelect, NativeSelectOptGroup, NativeSelectOption };
