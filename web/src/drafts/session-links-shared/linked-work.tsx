/** The one block every direction places somewhere: where it runs, then what's linked. */
import { Link2 } from "@compozy/ui/icons";

import { Button, Eyebrow, Icon } from "@compozy/ui";

import { SessionInspectorEmpty } from "@/systems/session/components/session-inspector-section";
import type { SessionLink, SessionRunsIn } from "@/systems/session/mocks/session-links-fixtures";

import { LinkList } from "./link-list";
import type { LinkRowLayout } from "./link-row";
import { RunsIn } from "./runs-in";

export interface LinkedWorkProps {
  links: readonly SessionLink[];
  runsIn: SessionRunsIn;
  layout?: LinkRowLayout;
  openMenuFor?: string;
  onLink: () => void;
  onUnlink: (link: SessionLink) => void;
}

function LinkButton({ onLink }: { onLink: () => void }) {
  return (
    <Button className="ml-auto" onClick={onLink} size="xs" type="button" variant="secondary">
      <Icon as={Link2} data-icon="inline-start" size="sm" />
      Link…
    </Button>
  );
}

export function LinkedWork({
  links,
  runsIn,
  layout,
  openMenuFor,
  onLink,
  onUnlink,
}: LinkedWorkProps) {
  return (
    <div className="flex min-w-0 flex-col gap-4" data-slot="session-linked-work">
      <RunsIn place={runsIn} />
      {links.length === 0 ? (
        <SessionInspectorEmpty
          action={<LinkButton onLink={onLink} />}
          icon={Link2}
          description="A worktree this session creates shows up here on its own. You can also link a pull request or a branch."
          title="Nothing linked yet"
        />
      ) : (
        <div className="flex min-w-0 flex-col gap-2">
          <div className="flex items-center gap-2 px-2.5">
            <Eyebrow>Linked to this session</Eyebrow>
            <span className="font-mono text-mono-id text-faint tabular-nums">{links.length}</span>
            <LinkButton onLink={onLink} />
          </div>
          <LinkList layout={layout} links={links} onUnlink={onUnlink} openMenuFor={openMenuFor} />
        </div>
      )}
    </div>
  );
}
