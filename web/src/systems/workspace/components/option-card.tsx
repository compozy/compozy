import * as React from "react";

import { FormSection } from "@agh/ui";

import { useOptionCardSlot } from "../hooks/use-option-card-slot";
import {
  OptionCardContext,
  type OptionCardContextValue,
  type OptionCardSize,
} from "./option-card-context";
import {
  OptionCardAction,
  OptionCardBody,
  OptionCardContent,
  OptionCardDescription,
  OptionCardIcon,
  OptionCardMeta,
  OptionCardTitle,
} from "./option-card-parts";

interface OptionCardHeaderProps {
  eyebrow?: React.ReactNode;
  right?: React.ReactNode;
}

function OptionCardHeaderSentinel(_: OptionCardHeaderProps): null {
  useOptionCardSlot("Header");
  return null;
}

function isHeaderElement(node: React.ReactNode): node is React.ReactElement<OptionCardHeaderProps> {
  return React.isValidElement(node) && node.type === OptionCardHeaderSentinel;
}

interface OptionCardProps extends Omit<
  React.ComponentProps<typeof FormSection>,
  "title" | "rightLabel" | "size"
> {
  size?: OptionCardSize;
}

function OptionCardRoot({ className, size = "comfortable", children, ...props }: OptionCardProps) {
  const ctx: OptionCardContextValue = { size };

  let headerEyebrow: React.ReactNode;
  let headerRight: React.ReactNode;
  const body: React.ReactNode[] = [];

  React.Children.forEach(children, child => {
    if (isHeaderElement(child)) {
      headerEyebrow = child.props.eyebrow;
      headerRight = child.props.right;
      return;
    }
    body.push(child);
  });

  return (
    <OptionCardContext.Provider value={ctx}>
      <FormSection
        {...props}
        data-slot="option-card"
        data-size={size}
        title={headerEyebrow}
        rightLabel={headerRight}
        className={className}
      >
        {body}
      </FormSection>
    </OptionCardContext.Provider>
  );
}

const OptionCard = Object.assign(OptionCardRoot, {
  Header: OptionCardHeaderSentinel,
  Body: OptionCardBody,
  Icon: OptionCardIcon,
  Content: OptionCardContent,
  Title: OptionCardTitle,
  Description: OptionCardDescription,
  Meta: OptionCardMeta,
  Action: OptionCardAction,
});

export { OptionCard };
export type { OptionCardHeaderProps, OptionCardProps, OptionCardSize };
export type { OptionCardIconProps, OptionCardTone } from "./option-card-parts";
