import type { ComponentProps } from "react";

import { StorySurface, StoryTopbarHost } from "@/storybook/story-layout";

import { LoopEditor } from "../editor/loop-editor";

export function EditorHarness({
  heightClass = "h-[880px]",
  ...args
}: ComponentProps<typeof LoopEditor> & { heightClass?: string }) {
  return (
    <StoryTopbarHost title="Editor">
      <StorySurface className={`flex ${heightClass} p-0`}>
        <LoopEditor {...args} />
      </StorySurface>
    </StoryTopbarHost>
  );
}
