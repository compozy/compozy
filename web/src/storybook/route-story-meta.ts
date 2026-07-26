export {
  StorybookRestartNoticeSetup,
  StorybookRouteCanvas,
  StorybookUserHomeDirSetup,
  StorybookWorkspaceSetup,
} from "./route-story";

export function appRouteParameters(path: string) {
  return {
    layout: "fullscreen" as const,
    router: {
      kind: "app" as const,
      initialEntries: [path],
    },
  };
}
