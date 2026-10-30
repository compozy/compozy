/**
 * Node inspector: with nothing selected, the flow summary; with a node, how a person arrives
 * ("Arrives from"), where they can go ("Leads to"), the other flows that show the same state
 * ("Also in") and the action that opens the prototype (or the external page).
 */
import { ExternalLinkIcon } from "lucide-react";
import type { ReactNode } from "react";

import { Eyebrow, Pill, buttonVariants, cn } from "@compozy/ui";

import type { FlowEntry, ResolvedEdge, ResolvedNode } from "#gallery/flow";
import { edgeKindStyle } from "#gallery/shell/flows/edge-style";
import { displayUrl, nodeName, prototypeSelection } from "#gallery/shell/flows/node-model";
import { flowById, occurrencesOf, plural, type Registry } from "#gallery/shell/registry-model";
import { SelectionLink } from "#gallery/shell/selection-link";

type InspectorProps = {
  flow: FlowEntry;
  node: ResolvedNode | null;
  registry: Registry;
  onSelectNode: (id: string) => void;
};

const rowClass =
  "flex w-full flex-col rounded-md px-1.5 py-1 text-left outline-none hover:bg-hover focus-visible:shadow-focus-ring";

function Well({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section aria-label={title} className="flex flex-col gap-1.5 rounded-lg bg-surface-2 p-3">
      <Eyebrow>{title}</Eyebrow>
      <ul className="flex flex-col gap-0.5">{children}</ul>
    </section>
  );
}

function Fixed({ children }: { children: ReactNode }) {
  return <li className="px-1.5 py-1 text-small-body text-muted">{children}</li>;
}

function EdgeRow({
  edge,
  other,
  onSelect,
}: {
  edge: ResolvedEdge;
  other: ResolvedNode | undefined;
  onSelect: (id: string) => void;
}) {
  if (!other) return null;
  return (
    <li>
      <button className={rowClass} onClick={() => onSelect(other.id)} type="button">
        <span className="text-small-body text-fg">
          {nodeName(other)}
          {other.kind === "prototype" ? (
            <span className="ml-1.5 font-mono text-mono-id tracking-mono-id text-subtle">
              {other.state}
            </span>
          ) : null}
        </span>
        <span className="text-small-body text-muted">
          {edgeKindStyle[edge.kind].label} · “{edge.on}”
        </span>
      </button>
    </li>
  );
}

function FlowSummary({ flow, registry }: { flow: FlowEntry; registry: Registry }) {
  const titleOf = (id: string) => registry.groups.find(group => group.id === id)?.title ?? id;
  return (
    <>
      <div className="flex flex-col gap-1">
        <Eyebrow>Flow</Eyebrow>
        <h2 className="text-item-title font-medium text-fg">{flow.title}</h2>
        {flow.description ? <p className="text-small-body text-muted">{flow.description}</p> : null}
      </div>
      <section className="flex flex-col gap-1.5 rounded-lg bg-surface-2 p-3">
        <Eyebrow>Owner spec</Eyebrow>
        <span className="text-small-body">{flow.spec ? titleOf(flow.spec) : "No spec"}</span>
        {flow.specs.length > 0 ? (
          <>
            <Eyebrow>Crosses</Eyebrow>
            <span className="flex flex-wrap gap-1">
              {flow.specs.map(spec => (
                <Pill key={spec}>{titleOf(spec)}</Pill>
              ))}
            </span>
          </>
        ) : null}
      </section>
      <p className="font-mono text-mono-id tracking-mono-id text-subtle">
        {plural(flow.nodes.length, "step")} · {plural(flow.edges.length, "transition")}
      </p>
      <p className="text-small-body text-muted">
        Select a step to see how people arrive there and where it leads.
      </p>
    </>
  );
}

function AlsoIn({
  registry,
  flow,
  node,
}: {
  registry: Registry;
  flow: FlowEntry;
  node: ResolvedNode;
}) {
  if (node.kind !== "prototype") return null;
  const elsewhere = occurrencesOf(registry, node.proto, node.state).filter(
    item => item.flow !== flow.id
  );
  if (elsewhere.length === 0) return null;
  return (
    <Well title="Also in">
      {elsewhere.map(item => (
        <li key={`${item.flow}:${item.node}`}>
          <SelectionLink
            className={rowClass}
            to={{ kind: "flow", flow: item.flow, node: item.node }}
          >
            <span className="text-small-body text-fg">
              {flowById(registry, item.flow)?.title ?? item.flow}
            </span>
            <span className="font-mono text-mono-id tracking-mono-id text-subtle">
              step {item.step} of {item.steps}
            </span>
          </SelectionLink>
        </li>
      ))}
    </Well>
  );
}

export function FlowInspector({ flow, node, registry, onSelectNode }: InspectorProps) {
  if (!node) return <FlowSummary flow={flow} registry={registry} />;
  const byId = (id: string) => flow.nodes.find(item => item.id === id);
  const incoming = flow.edges.filter(edge => edge.to === node.id);
  const outgoing = flow.edges.filter(edge => edge.from === node.id);
  return (
    <>
      <div className="flex flex-col gap-1">
        <Eyebrow>
          Step {node.step} of {flow.nodes.length}
        </Eyebrow>
        <h2 className="text-item-title font-medium text-fg">{nodeName(node)}</h2>
        <span className="font-mono text-mono-id tracking-mono-id break-all text-subtle">
          {node.kind === "prototype"
            ? [node.proto, node.state, node.route].filter(Boolean).join(" · ")
            : node.url
              ? displayUrl(node.url)
              : "outside the web app"}
        </span>
        {node.kind === "prototype" ? (
          <span className="text-small-body text-muted">{node.stateLabel}</span>
        ) : null}
      </div>
      <Well title="Arrives from">
        {node.start ? <Fixed>Start of the flow</Fixed> : null}
        {incoming.map(edge => (
          <EdgeRow
            edge={edge}
            key={`${edge.from}:${edge.on}`}
            onSelect={onSelectNode}
            other={byId(edge.from)}
          />
        ))}
      </Well>
      <Well title="Leads to">
        {outgoing.map(edge => (
          <EdgeRow
            edge={edge}
            key={`${edge.to}:${edge.on}`}
            onSelect={onSelectNode}
            other={byId(edge.to)}
          />
        ))}
        {node.end ? <Fixed>End of the flow</Fixed> : null}
      </Well>
      <AlsoIn flow={flow} node={node} registry={registry} />
      {node.kind === "prototype" ? (
        <>
          <SelectionLink
            className={cn(buttonVariants({ size: "default" }), "w-full")}
            data-testid="flow-open-prototype"
            to={prototypeSelection(node)}
          >
            Open prototype in this state
          </SelectionLink>
          <p className="text-small-body text-muted">Double-click the step or press Shift+Enter.</p>
        </>
      ) : node.url ? (
        <a
          className={cn(buttonVariants({ variant: "secondary" }), "w-full")}
          href={node.url}
          rel="noreferrer"
          target="_blank"
        >
          Open page
          <ExternalLinkIcon aria-hidden="true" />
        </a>
      ) : null}
    </>
  );
}
