import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { Action, ActionPanel } from "../actions.js";
import { List } from "../list.js";
import { useCachedPromise } from "../hooks/use-cached-promise.js";
import { useNavigation } from "../hooks/use-navigation.js";
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
