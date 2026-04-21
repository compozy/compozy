import { describe, expect, it } from "vitest";

const uiMain = (await import("../.storybook/main")).default;
const uiPreviewModule = await import("../.storybook/preview");
const { storybookDecorators, themeDecorator } = uiPreviewModule;
const uiPreview = uiPreviewModule.default;

describe("packages/ui Storybook config", () => {
  it("uses the expected story glob, addons, and framework", () => {
    expect(uiMain.stories).toEqual(["../src/**/*.stories.@(ts|tsx)"]);
    expect(uiMain.addons).toEqual([
      "@storybook/addon-docs",
      "@storybook/addon-a11y",
      "@storybook/addon-themes",
    ]);
    expect(uiMain.framework).toEqual({
      name: "@storybook/react-vite",
      options: {},
    });
  });

  it("registers the theme decorator and the shared preview parameters", () => {
    expect(storybookDecorators).toEqual([themeDecorator]);
    expect(uiPreview.decorators).toEqual(storybookDecorators);
    expect(uiPreview.parameters?.backgrounds).toEqual({ disable: true });
    expect(uiPreview.parameters?.controls).toEqual({ expanded: true });
  });
});
