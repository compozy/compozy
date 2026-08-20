import { pathToFileURL } from "node:url";

import { Extension, type ExtensionOptions } from "@compozy/extension-sdk";
import { Action, ActionPanel, List, registerReactViews } from "@compozy/extension-react";

function BrowserView() {
  return (
    <List searchBarPlaceholder="Search…" complete>
      <List.Section title="Results">
        <List.Item
          id="welcome"
          title="Welcome"
          actions={
            <ActionPanel>
              <Action title="Run" onAction={() => undefined} />
            </ActionPanel>
          }
        />
      </List.Section>
    </List>
  );
}

export function createExtension(options: ExtensionOptions = {}): Extension {
  const extension = new Extension(
    {
      name: "__EXTENSION_NAME__",
      version: "0.1.0",
      description: "Provide a programmable command-palette view",
      subprocess: { command: "node", args: ["index.js"] },
      capabilities: { provides: ["view.provider"] },
      permissions: { requires: ["view/patch"] },
      resources: {
        cmd_palette: {
          commands: [
            {
              id: "browser",
              title: "Open browser",
              icon: "search",
              action: { kind: "view", view: "browser" },
            },
          ],
          views: [{ id: "browser", title: "Browser", kind: "list", program: true }],
        },
      },
    },
    options
  );
  return registerReactViews(extension, { browser: BrowserView });
}

const entryPoint = process.argv[1];
if (entryPoint && import.meta.url === pathToFileURL(entryPoint).href) {
  void createExtension().start();
}
