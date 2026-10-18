import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { sessionOriginView } from "../../lib/session-origin";
import { continuedSessionFixture } from "../../mocks/derive-fixtures";
import type { SessionPayload } from "../../types";
import { SessionContinueDivider, SessionContinueEmptyChild } from "../session-continue-divider";
import { SessionStatusLine } from "../session-status-line";

const SOURCE_TITLE = "Refactor flaky manager tests";
const continued = continuedSessionFixture();
const forked: SessionPayload = {
  ...continued,
  agent_name: "claude",
  lineage: { ...continued.lineage!, kind: "fork" },
};

const continueOrigin = sessionOriginView(continued, { title: SOURCE_TITLE })!;
const forkOrigin = sessionOriginView(forked, { title: SOURCE_TITLE })!;
const goneOrigin = sessionOriginView(continued, null)!;

/**
 * Where a derived session came from (lineage board §01–§02): a neutral pill
 * after the provider in the status line and one hairline divider before the
 * child's own first message. Both open the source while it is readable and
 * keep their words, without a link, once it is gone.
 */
function OriginGallery() {
  const onOpenSource = fn();
  return (
    <div className="flex max-w-3xl flex-col gap-8 p-8 text-fg">
      <section className="flex flex-col gap-3">
        <SessionStatusLine
          onOpenOriginSource={onOpenSource}
          origin={continueOrigin}
          session={continued}
        />
        <SessionStatusLine onOpenOriginSource={onOpenSource} origin={forkOrigin} session={forked} />
        <SessionStatusLine origin={goneOrigin} session={continued} />
        {/* VC-18: a 640px head keeps the pill's verb and truncates its subject. */}
        <div className="w-[320px] overflow-hidden">
          <SessionStatusLine
            onOpenOriginSource={onOpenSource}
            origin={forkOrigin}
            session={forked}
          />
        </div>
      </section>
      <section className="flex flex-col gap-6">
        <SessionContinueDivider onOpenSource={onOpenSource} origin={continueOrigin} />
        <SessionContinueDivider origin={goneOrigin} />
        <SessionContinueEmptyChild onOpenSource={onOpenSource} origin={continueOrigin} />
      </section>
    </div>
  );
}

const meta: Meta<typeof OriginGallery> = {
  title: "systems/session/components/SessionOrigin",
  component: OriginGallery,
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** VC-16, VC-18, VC-19, VC-20: pill (link, fork, source gone, narrow) and divider states. */
export const Gallery: Story = {};
