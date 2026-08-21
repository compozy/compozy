import { pathToFileURL } from "node:url";
import { useState } from "react";

import { Extension, type ExtensionOptions } from "@compozy/extension-sdk";
import {
  Action,
  ActionPanel,
  Form,
  List,
  registerReactViews,
  showToast,
} from "@compozy/extension-react";

interface BrowserProps {
  mode?: "fast" | "slow" | "crash";
}

function NotesBrowser({ mode = "fast" }: BrowserProps) {
  const [query, setQuery] = useState("");
  const [chip, setChip] = useState("all");
  const notes = ["Release notes", "Standup follow-ups"].filter(note =>
    note.toLowerCase().includes(query.toLowerCase())
  );
  const updateSearch = async (value: string) => {
    if (mode === "crash") throw new Error("fixture crash requested");
    if (mode === "slow") await new Promise(resolve => setTimeout(resolve, 3_100));
    setQuery(value);
  };
  return (
    <List
      searchText={query}
      searchBarPlaceholder="Search notes…"
      chips={[
        { id: "all", label: "All", count: 2 },
        { id: "inbox", label: "Inbox", count: 1 },
      ]}
      activeChip={chip}
      complete
      onSearchTextChange={updateSearch}
      onChipToggle={value => setChip(value ?? "all")}
    >
      <List.Section title="Notes">
        {notes.map(note => (
          <List.Item
            key={note}
            id={note.toLowerCase().replaceAll(" ", "-")}
            title={note}
            detail={
              <List.Item.Detail
                markdown={`## ${note}\n\nFixture detail rendered by the extension.`}
                metadata={[{ label: "Folder", value: chip }]}
              />
            }
            actions={
              <ActionPanel>
                <Action.Push title="Edit" target={<NoteForm title={note} />} />
                <Action
                  title="Delete"
                  style="destructive"
                  confirmation={{ title: "Delete note?", confirm: "Delete" }}
                  onAction={() => showToast({ tone: "success", message: "Deleted" })}
                />
              </ActionPanel>
            }
          />
        ))}
      </List.Section>
      {notes.length === 0 ? <List.EmptyView title="No notes" hint="Try another search." /> : null}
    </List>
  );
}

function NoteForm({ title }: { title: string }) {
  return (
    <Form onSubmit={() => showToast({ tone: "success", message: "Saved" })}>
      <Form.TextField id="title" label="Title" defaultValue={title} required />
      <Form.Dropdown id="folder" label="Folder" options={["Inbox", "Archive"]} />
      <ActionPanel>
        <Action.SubmitForm title="Save" />
      </ActionPanel>
    </Form>
  );
}

export function createExtension(options: ExtensionOptions = {}): Extension {
  const extension = new Extension(
    {
      name: "notes-ts",
      version: "0.1.0",
      description: "Programmable command-palette integration fixture",
      subprocess: { command: "node", args: ["index.js"] },
      capabilities: { provides: ["view.provider"] },
      permissions: { requires: ["view/patch"] },
      resources: {
        cmd_palette: {
          commands: [
            {
              id: "browser",
              title: "Browse notes",
              section: "Notes",
              icon: "notebook",
              action: { kind: "view", view: "browser" },
            },
          ],
          views: [{ id: "browser", title: "Browse notes", kind: "list", program: true }],
        },
      },
    },
    options
  );
  return registerReactViews(extension, { browser: NotesBrowser });
}

const entryPoint = process.argv[1];
if (entryPoint && import.meta.url === pathToFileURL(entryPoint).href) {
  void createExtension().start();
}
