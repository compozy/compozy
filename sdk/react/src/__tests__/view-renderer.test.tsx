import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { Action, ActionPanel } from "../actions.js";
import { Grid } from "../detail-grid.js";
import { Form } from "../form.js";
import { HandlerRegistry } from "../handler-registry.js";
import type { HostNode } from "../host-types.js";
import { List } from "../list.js";
import { useCachedPromise } from "../hooks/use-cached-promise.js";
import { useNavigation } from "../hooks/use-navigation.js";
import { serializeView } from "../serializer.js";
import { showToast } from "../view-effects.js";
import { ViewRenderer } from "../view-renderer.js";

describe("ViewRenderer", () => {
  it("renders the Notes browser vocabulary through the persistent reconciler [UT-169]", () => {
    const renderer = createRenderer();
    const frame = renderer.open(<NotesBrowser />);

    expect(frame.payload).toMatchObject({
      view: "v1",
      chrome: {
        filtering: false,
        on_search: expect.stringMatching(/^handler_/),
      },
      chips: [{ id: "inbox", label: "Inbox", count: 1 }],
      sections: [
        {
          title: "Today",
          rows: [
            {
              id: "note-1",
              title: "First note",
              actions: [
                {
                  title: "Delete",
                  destructive: true,
                  confirmation: { title: "Delete note?", confirm: "Delete" },
                  handler: expect.stringMatching(/^handler_/),
                },
              ],
            },
          ],
        },
      ],
    });
    renderer.close();
  });

  it("keeps surviving handler ids and omits removed handlers [UT-170]", async () => {
    const renderer = createRenderer();
    const first = renderer.open(<HandlerIdentityView />);
    const [removeID, survivingID] = first.handlers;

    const second = await renderer.event(removeID!, [], 1, 1, new AbortController().signal);

    expect(second?.handlers).toEqual([survivingID]);
    expect(second?.handlers).not.toContain(removeID);
    renderer.close();
  });

  it("batches state bursts into one patch and suppresses identical content [UT-171]", async () => {
    const published = vi.fn();
    const scheduled: (() => void)[] = [];
    const renderer = createRenderer({
      publish: published,
      scheduleFrame: callback => scheduled.push(callback),
    });
    const first = renderer.open(<BurstView />);
    const [burstID, noopID] = first.handlers;

    const patch = await renderer.event(burstID!, [], 1, 1, new AbortController().signal);
    expect(patch?.patch?.ops).toEqual([
      expect.objectContaining({ op: "replace", path: "", value: expect.any(Object) }),
    ]);
    const patchedPayload = patch?.patch?.ops[0]?.value as
      | { sections: { rows: { title: string }[] }[] }
      | undefined;
    expect(patchedPayload?.sections[0]?.rows[0]?.title).toBe("Count 3");
    expect(await renderer.event(noopID!, [], 2, 2, new AbortController().signal)).toBeUndefined();

    for (const callback of scheduled) callback();
    expect(published).not.toHaveBeenCalled();
    renderer.close();
  });

  it("warns on starvation and aborts cached work when the session closes [UT-172]", async () => {
    const warnings: string[] = [];
    const readings = [0, 75];
    const renderer = createRenderer({
      diagnostics: { warn: message => warnings.push(message) },
      now: () => readings.shift() ?? 75,
      starvationBudgetMS: 50,
    });
    let observedSignal: AbortSignal | undefined;
    const frame = renderer.open(
      <AbortableView
        observe={signal => {
          observedSignal = signal;
        }}
      />
    );
    await Promise.resolve();

    await renderer.event(frame.handlers[0]!, [], 1, 1, new AbortController().signal);
    expect(warnings).toEqual([expect.stringContaining("blocked the event loop for 75ms")]);
    renderer.close();
    expect(observedSignal?.aborted).toBe(true);
  });

  it("preserves parent component state across push and pop navigation [UT-169]", async () => {
    const renderer = createRenderer();
    const first = renderer.open(<NavigationParent />);

    const incremented = await renderer.event(
      handlerForAction(first, "Increment"),
      [],
      1,
      1,
      new AbortController().signal
    );
    expect(framePayload(incremented).sections?.[0]?.rows[0]?.title).toBe("Count 1");

    const child = await renderer.event(
      handlerForAction(incremented, "Open child"),
      [],
      2,
      2,
      new AbortController().signal
    );
    expect(framePayload(child).sections?.[0]?.rows[0]?.title).toBe("Child");

    const restored = await renderer.event(
      handlerForAction(child, "Back"),
      [],
      3,
      3,
      new AbortController().signal
    );
    expect(framePayload(restored).sections?.[0]?.rows[0]?.title).toBe("Count 1");
    renderer.close();
  });

  it("echoes controlled search text with the host event_count", async () => {
    const renderer = createRenderer();
    const first = renderer.open(<ControlledSearchView />);
    const reply = await renderer.event(
      chromeHandler(first, "on_search"),
      ["hello", 4],
      1,
      1,
      new AbortController().signal
    );

    expect(framePayload(reply).chrome).toMatchObject({
      search_text: "hello",
      event_count: 4,
    });
    renderer.close();
  });

  it("maps Form validation onto field error and echoes field event_count", async () => {
    const renderer = createRenderer();
    const first = renderer.open(<ValidatedFormView />);
    expect(first.payload?.form?.fields[0]).toMatchObject({
      id: "title",
      error: "Required",
    });

    const onChange = first.payload?.form?.fields[0]?.on_change;
    if (!onChange) throw new Error("missing form on_change handler");
    const reply = await renderer.event(onChange, ["Ada", 3], 1, 1, new AbortController().signal);
    expect(framePayload(reply).form?.fields[0]).toMatchObject({
      default: "Ada",
      event_count: 3,
    });
    renderer.close();
  });

  it("serializes Grid list-parity chrome, chips, pagination, and columns", () => {
    const renderer = createRenderer();
    const frame = renderer.open(<ParityGridView />);

    expect(frame.payload).toMatchObject({
      chips: [{ id: "inbox", label: "Inbox", count: 1 }],
      chrome: {
        columns: 3,
        filtering: false,
        active_chip: "inbox",
        pagination: { has_more: true, page_size: 20 },
        on_search: expect.stringMatching(/^handler_/),
        on_chip: expect.stringMatching(/^handler_/),
        on_selection: expect.stringMatching(/^handler_/),
        on_load_more: expect.stringMatching(/^handler_/),
      },
      grid: {
        sections: [{ tiles: [{ id: "note-1", title: "First note", image: { emoji: "📄" } }] }],
      },
    });
    renderer.close();
  });

  it("serializes CopyToClipboard as a host-target copy action", () => {
    const renderer = createRenderer();
    const frame = renderer.open(<CopyActionView />);
    const action = frame.payload?.sections?.[0]?.rows[0]?.actions?.[0];

    expect(action).toMatchObject({
      title: "Copy",
      action: { kind: "copy", args: { content: "clipboard text" } },
    });
    expect(action?.handler).toBeUndefined();
    renderer.close();
  });

  it("keeps previous rows and sets isLoading while a refetch is in flight", async () => {
    notesFetch.sequence = [defer(), defer()];
    notesFetch.calls = 0;
    const renderer = createRenderer();
    const first = renderer.open(<NotesSearchView />);
    expect(first.payload?.chrome?.is_loading).toBe(true);

    notesFetch.sequence[0]?.resolve();
    await renderer.event(handlerForAction(first, "Probe"), [], 1, 1, new AbortController().signal);
    await yieldEventLoop();
    const settled = await renderer.event(
      handlerForAction(first, "Probe"),
      [],
      2,
      2,
      new AbortController().signal
    );
    expect(framePayload(settled).chrome?.is_loading).toBeUndefined();
    expect(framePayload(settled).sections?.[0]?.rows[0]?.title).toBe("inbox");

    const reply = await renderer.event(
      chromeHandler(settled, "on_search"),
      ["docs", 2],
      3,
      3,
      new AbortController().signal
    );
    expect(framePayload(reply).chrome).toMatchObject({
      is_loading: true,
      search_text: "docs",
      event_count: 2,
    });
    expect(framePayload(reply).sections?.[0]?.rows[0]?.title).toBe("inbox");
    renderer.close();
  });

  it("appends the next page from useCachedPromise loadMore", async () => {
    const firstPagePublished = defer();
    let settled: Awaited<ReturnType<ViewRenderer["event"]>> = undefined;
    const renderer = createRenderer({
      publish: frame => {
        settled = frame;
        firstPagePublished.resolve();
      },
    });
    renderer.open(<PagedNotesView />);
    await firstPagePublished.promise;
    expect.assert(settled);
    expect(rowTitles(settled)).toEqual(["one"]);

    const reply = await renderer.event(
      chromeHandler(settled, "on_load_more"),
      [],
      3,
      3,
      new AbortController().signal
    );
    expect(rowTitles(reply)).toEqual(["one", "two"]);
    renderer.close();
  });

  it("does not publish stale state or effects from a canceled handler [UT-173]", async () => {
    const published = vi.fn();
    const scheduled: (() => void)[] = [];
    const renderer = createRenderer({
      publish: published,
      scheduleFrame: callback => scheduled.push(callback),
    });
    const first = renderer.open(<CanceledPublishView />);
    const controller = new AbortController();
    const pending = renderer.event(handlerForAction(first, "Slow"), [], 1, 1, controller.signal);
    controller.abort();

    expect(await pending).toBeUndefined();
    await new Promise<void>(resolve => {
      setTimeout(resolve, 40);
    });
    for (const callback of scheduled) callback();
    expect(published).not.toHaveBeenCalled();
    renderer.close();
  });

  it("closes the handler frame when serialization throws", () => {
    const registry = new HandlerRegistry();
    const broken = hostNode("view-grid", {}, [
      hostNode("view-grid-item", { id: "broken", title: "Broken", image: {} }),
    ]);
    expect(() => serializeView([broken], registry)).toThrow(/url, token, or emoji/);

    const recovered = serializeView(
      [hostNode("view-list", {}, [hostNode("view-list-item", { id: "ok", title: "Recovered" })])],
      registry
    );
    expect(recovered.payload.sections?.[0]?.rows[0]?.title).toBe("Recovered");
  });
});

function NotesBrowser() {
  return (
    <List chips={[{ id: "inbox", label: "Inbox", count: 1 }]} onSearchTextChange={() => undefined}>
      <List.Section title="Today">
        <List.Item
          id="note-1"
          title="First note"
          actions={
            <ActionPanel>
              <Action
                title="Delete"
                style="destructive"
                confirmation={{ title: "Delete note?", confirm: "Delete" }}
                onAction={() => undefined}
              />
            </ActionPanel>
          }
        />
      </List.Section>
    </List>
  );
}

function HandlerIdentityView() {
  const [showRemove, setShowRemove] = useState(true);
  return (
    <List>
      <List.Item
        id="row"
        title="Row"
        actions={
          <ActionPanel>
            {showRemove ? <Action title="Remove" onAction={() => setShowRemove(false)} /> : null}
            <Action title="Survive" onAction={() => undefined} />
          </ActionPanel>
        }
      />
    </List>
  );
}

function BurstView() {
  const [count, setCount] = useState(0);
  return (
    <List>
      <List.Item
        id="count"
        title={`Count ${count}`}
        actions={
          <ActionPanel>
            <Action
              title="Burst"
              onAction={() => {
                setCount(value => value + 1);
                setCount(value => value + 1);
                setCount(value => value + 1);
              }}
            />
            <Action title="Noop" onAction={() => setCount(value => value)} />
          </ActionPanel>
        }
      />
    </List>
  );
}

function AbortableView({ observe }: { observe: (signal: AbortSignal) => void }) {
  useCachedPromise(async signal => {
    observe(signal);
    await new Promise<void>(resolve =>
      signal.addEventListener("abort", () => resolve(), { once: true })
    );
    return [];
  }, []);
  return (
    <List>
      <List.Item
        id="wait"
        title="Waiting"
        actions={
          <ActionPanel>
            <Action title="Block" onAction={() => undefined} />
          </ActionPanel>
        }
      />
    </List>
  );
}

function NavigationParent() {
  const [count, setCount] = useState(0);
  const { push } = useNavigation();
  return (
    <List>
      <List.Item
        id="parent"
        title={`Count ${count}`}
        actions={
          <ActionPanel>
            <Action title="Increment" onAction={() => setCount(value => value + 1)} />
            <Action title="Open child" onAction={() => push(<NavigationChild />)} />
          </ActionPanel>
        }
      />
    </List>
  );
}

function NavigationChild() {
  const { pop } = useNavigation();
  return (
    <List>
      <List.Item
        id="child"
        title="Child"
        actions={
          <ActionPanel>
            <Action title="Back" onAction={pop} />
          </ActionPanel>
        }
      />
    </List>
  );
}

function ControlledSearchView() {
  const [search, setSearch] = useState("");
  return (
    <List searchText={search} onSearchTextChange={value => setSearch(value)}>
      <List.Item id="row" title={search || "empty"} />
    </List>
  );
}

function ValidatedFormView() {
  const [title, setTitle] = useState("");
  return (
    <Form validation={{ title: "Required" }}>
      <Form.TextField
        id="title"
        label="Title"
        defaultValue={title}
        onChange={value => setTitle(String(value))}
      />
    </Form>
  );
}

function ParityGridView() {
  return (
    <Grid
      chips={[{ id: "inbox", label: "Inbox", count: 1 }]}
      activeChip="inbox"
      columns={3}
      pagination={{ hasMore: true, pageSize: 20, onLoadMore: () => undefined }}
      onSearchTextChange={() => undefined}
      onChipToggle={() => undefined}
      onSelectionChange={() => undefined}
    >
      <Grid.Item id="note-1" title="First note" image={{ emoji: "📄" }} />
    </Grid>
  );
}

function CopyActionView() {
  return (
    <List>
      <List.Item
        id="row"
        title="Row"
        actions={
          <ActionPanel>
            <Action.CopyToClipboard title="Copy" content="clipboard text" />
          </ActionPanel>
        }
      />
    </List>
  );
}

const notesFetch = {
  sequence: [] as Deferred[],
  calls: 0,
  async run(): Promise<string[]> {
    notesFetch.calls += 1;
    const gate = notesFetch.sequence[notesFetch.calls - 1];
    if (gate) await gate.promise;
    return [notesFetch.calls === 1 ? "inbox" : "docs"];
  },
};

function NotesSearchView() {
  const [query, setQuery] = useState("inbox");
  const { data, isLoading } = useCachedPromise(async () => notesFetch.run(), [query]);
  return (
    <List isLoading={isLoading} searchText={query} onSearchTextChange={value => setQuery(value)}>
      {(data ?? ["pending"]).map(title => (
        <List.Item
          key={title}
          id={title}
          title={title}
          actions={
            <ActionPanel>
              <Action title="Probe" onAction={() => undefined} />
            </ActionPanel>
          }
        />
      ))}
    </List>
  );
}

async function fetchNotePages(_signal: AbortSignal, page: number): Promise<string[]> {
  return page === 0 ? ["one"] : ["two"];
}

function PagedNotesView() {
  const { data, isLoading, loadMore } = useCachedPromise(fetchNotePages, []);
  return (
    <List
      isLoading={isLoading}
      pagination={{ hasMore: true, pageSize: 1, onLoadMore: () => loadMore() }}
    >
      {(data ?? ["pending"]).map(title => (
        <List.Item
          key={title}
          id={title}
          title={title}
          actions={
            <ActionPanel>
              <Action title="Probe" onAction={() => undefined} />
            </ActionPanel>
          }
        />
      ))}
    </List>
  );
}

function CanceledPublishView() {
  const [title, setTitle] = useState("Initial");
  return (
    <List>
      <List.Item
        id="row"
        title={title}
        actions={
          <ActionPanel>
            <Action
              title="Slow"
              onAction={async () => {
                await new Promise<void>(resolve => {
                  setTimeout(resolve, 20);
                });
                setTitle("Stale");
                await showToast({ tone: "info", message: "stale" });
              }}
            />
          </ActionPanel>
        }
      />
    </List>
  );
}

function framePayload(
  frame: Awaited<ReturnType<ViewRenderer["event"]>> | ReturnType<ViewRenderer["open"]>
) {
  if (!frame) throw new Error("expected a view frame");
  if (frame.payload) return frame.payload;
  const replacement = frame.patch?.ops.find(operation => operation.op === "replace");
  if (!replacement || !("value" in replacement))
    throw new Error("expected a root replacement patch");
  return replacement.value as unknown as NonNullable<ReturnType<ViewRenderer["open"]>["payload"]>;
}

function chromeHandler(
  frame: Awaited<ReturnType<ViewRenderer["event"]>> | ReturnType<ViewRenderer["open"]>,
  key: "on_search" | "on_chip" | "on_load_more"
): string {
  const handler = framePayload(frame).chrome?.[key];
  if (!handler) throw new Error(`missing chrome handler ${key}`);
  return handler;
}

function rowTitles(
  frame: Awaited<ReturnType<ViewRenderer["event"]>> | ReturnType<ViewRenderer["open"]>
): string[] {
  return framePayload(frame).sections?.[0]?.rows.map(row => row.title) ?? [];
}

function hostNode(
  type: HostNode["type"],
  props: HostNode["props"],
  children: HostNode[] = []
): HostNode {
  return { type, props, children, handlerIDs: new Map(), hidden: false };
}

interface Deferred {
  promise: Promise<void>;
  resolve: () => void;
}

function defer(): Deferred {
  let resolve!: () => void;
  const promise = new Promise<void>(done => {
    resolve = done;
  });
  return { promise, resolve };
}

async function yieldEventLoop(): Promise<void> {
  await new Promise<void>(resolve => {
    setTimeout(resolve, 0);
  });
}

function handlerForAction(
  frame: Awaited<ReturnType<ViewRenderer["event"]>> | ReturnType<ViewRenderer["open"]>,
  title: string
): string {
  const payload = framePayload(frame);
  const action = payload.sections
    ?.flatMap(section => section.rows)
    .flatMap(row => row.actions ?? [])
    .find(candidate => candidate.title === title);
  if (!action?.handler) throw new Error(`missing handler for action ${title}`);
  return action.handler;
}

function createRenderer(
  overrides: Partial<ConstructorParameters<typeof ViewRenderer>[0]> = {}
): ViewRenderer {
  return new ViewRenderer({
    viewSession: "vs_test",
    viewID: "ext.notes.browser",
    signal: new AbortController().signal,
    publish: () => undefined,
    ...overrides,
  });
}
