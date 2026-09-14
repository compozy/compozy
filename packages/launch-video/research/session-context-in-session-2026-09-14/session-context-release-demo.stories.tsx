import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { Button, useTopbarSlot } from "@compozy/ui";
import { PanelRight } from "lucide-react";
import { SessionThread } from "@/components/assistant-ui/session-thread";
import { SessionChatRuntimeProvider } from "../session-chat-runtime-provider";
import { SessionContextControl } from "../session-context-control";
import { SessionInspector } from "../session-inspector";
import { deriveSessionContext } from "../../lib/session-context";
import { sessionContextFixture, sessionContextTurnsFixture } from "../../mocks/context-fixtures";
import { primarySessionFixture } from "../../mocks";
import { OsWindowFrame } from "@/systems/os/components/os-window-frame";
import { DesktopShell, buildDeskItems } from "@/systems/os/components/stories/_desktop";

// Capture-only composition: production window, transcript, composer, ring and inspector.
// Storybook supplies demonstration data; no live provider request is made.
function SessionDemoBody() {
  const [open, setOpen] = useState(false);
  const context = deriveSessionContext(sessionContextFixture);
  useTopbarSlot({ actions: <Button variant="ghost" size="icon-xs" aria-label={open ? "Close context sidebar" : "Open context sidebar"} onClick={() => setOpen(!open)}><PanelRight /></Button> });
  return <div className="flex min-h-0 flex-1 overflow-hidden">
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <SessionThread sessionId={primarySessionFixture.id} workspaceId={primarySessionFixture.workspace_id!}
        agentName={primarySessionFixture.agent_name} canPrompt isSessionRunning={false}
        liveDataEnabled={false} onCancelPrompt={() => undefined}
        runtimeControl={<Button variant="ghost" size="sm">Claude Code</Button>}
        contextControl={<SessionContextControl context={context} open={open} onOpen={() => setOpen(true)} />} />
    </div>
    {open && <SessionInspector context={context} turns={sessionContextTurnsFixture}
      drawerOpen onDrawerOpenChange={setOpen}
      usage={{ tokensIn:128400, tokensOut:24900, totalTokens:153300, cacheReadTokens:102400,
        cacheWriteTokens:12800, costUsd:18.42, costCurrency:"USD", costStatus:"actual", costSource:"agent_reported", turnCount:12 }} />}
  </div>;
}
function SessionContextReleaseDemo() {
  return <DesktopShell dockItems={buildDeskItems({open:["sessions"]})}>
    <OsWindowFrame title="Launch readiness" focused style={{position:"absolute",left:12,right:12,top:12,bottom:84}}>
      <SessionChatRuntimeProvider sessionId={primarySessionFixture.id} workspaceId={primarySessionFixture.workspace_id!} liveTailEnabled={false}>
        <SessionDemoBody />
      </SessionChatRuntimeProvider>
    </OsWindowFrame>
  </DesktopShell>;
}
const meta = { title:"release/SessionContext", component:SessionContextReleaseDemo, parameters:{layout:"fullscreen"} } satisfies Meta<typeof SessionContextReleaseDemo>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Session: Story = {};
