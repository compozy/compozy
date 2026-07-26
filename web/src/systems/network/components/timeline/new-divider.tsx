import { Separator } from "@agh/ui";

export interface NewDividerProps {
  label?: string;
}

export function NewDivider({ label = "NEW" }: NewDividerProps) {
  return (
    <Separator
      aria-label="New messages"
      className="my-4 px-5"
      data-testid="network-timeline-new-divider"
      label={label}
      lineClassName="data-horizontal:flex-1"
      tone="accent"
    />
  );
}
