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

/**
 * Story `beforeEach` that swallows `window.open`: a sign-in opens a browser tab by itself, and the
 * canvas must not leave the story. Restores the real `window.open` afterwards.
 */
export function holdWindowOpen() {
  const open = window.open;
  window.open = () => null;
  return () => {
    window.open = open;
  };
}
