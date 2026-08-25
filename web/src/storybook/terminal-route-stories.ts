export const terminalRouteStories = [
  {
    system: "terminal",
    routePath: "/terminal",
    storybookPath: "/terminal",
    title: "systems/terminal/routes/Terminal",
    storyName: "Controlled",
  },
  {
    system: "terminal",
    routePath: "/terminal/",
    storybookPath: "/terminal/",
    title: "systems/terminal/routes/Terminal",
    storyName: "Empty",
  },
  {
    system: "terminal",
    routePath: "/terminal/$terminalId",
    storybookPath: "/terminal/term-dev-server",
    title: "systems/terminal/routes/Terminal",
    storyName: "Controlled",
  },
] as const;
