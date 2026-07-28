import type { CSSProperties } from "react";
import { AbsoluteFill, interpolate, useCurrentFrame } from "remotion";
import { TOKENS } from "./tokens";
import { CONVERSATION, activeAgentAt, COMPOSITION_HEIGHT, COMPOSITION_WIDTH } from "./data";
import type { Conversation } from "./data";
import { ChatFrame } from "./components/chat-frame";
import { ChatHeader } from "./components/chat-header";
import { SidebarRail } from "./components/sidebar-rail";
import { MessageAgent } from "./components/message-agent";
import { ToolCard } from "./components/tool-card";
import { Composer } from "./components/composer";

interface HeroChatViewProps {
  staticMode?: boolean;
  conv?: Conversation;
}

const SHELL_STYLE: CSSProperties = {
  padding: 16,
  width: COMPOSITION_WIDTH,
  height: COMPOSITION_HEIGHT,
  backgroundColor: "transparent",
};
const LAYOUT_STYLE: CSSProperties = {
  display: "flex",
  flexDirection: "row",
  flex: 1,
  minHeight: 0,
};
const MAIN_STYLE: CSSProperties = {
  display: "flex",
  flexDirection: "column",
  flex: 1,
  minWidth: 0,
  backgroundColor: TOKENS.surfacePanel,
};
const SCROLL_WRAP_STYLE: CSSProperties = {
  flex: 1,
  minHeight: 0,
  overflow: "hidden",
  position: "relative",
  backgroundColor: TOKENS.canvas,
};

export function HeroChatComposition({
  staticMode = false,
  conv = CONVERSATION,
}: HeroChatViewProps = {}) {
  const frame = useCurrentFrame();

  const chromeOpacity = staticMode
    ? 1
    : interpolate(
        frame,
        [conv.chromeIn.start, conv.chromeIn.end, conv.chromeOut.start, conv.chromeOut.end],
        [0, 1, 1, 0],
        { extrapolateLeft: "clamp", extrapolateRight: "clamp" }
      );
  const bodyOpacity = staticMode
    ? 1
    : interpolate(
        frame,
        [conv.chromeIn.start + 4, conv.chromeIn.end + 4, conv.bodyOut.start, conv.bodyOut.end],
        [0, 1, 1, 0],
        { extrapolateLeft: "clamp", extrapolateRight: "clamp" }
      );
  const activeAgent = staticMode
    ? conv.agents[conv.agents.length - 1].id
    : activeAgentAt(frame, conv);

  // Bottom-anchored stack: new messages appear at the bottom,
  // older ones push up (and clip at top via overflow:hidden).
  const scrollInner: CSSProperties = {
    position: "absolute",
    inset: 0,
    padding: "16px 0 12px 0",
    opacity: bodyOpacity,
    display: "flex",
    flexDirection: "column",
    justifyContent: "flex-end",
  };

  return (
    <AbsoluteFill style={SHELL_STYLE}>
      <ChatFrame opacity={chromeOpacity}>
        <ChatHeader activeAgent={activeAgent} sessionName={conv.session.name} />
        <div style={LAYOUT_STYLE}>
          <SidebarRail activeAgent={activeAgent} />
          <div style={MAIN_STYLE}>
            <div style={SCROLL_WRAP_STYLE}>
              <div style={scrollInner}>
                {conv.agents.map(a => (
                  <MessageAgent
                    key={`${a.id}-${a.timestamp}`}
                    agent={a.id}
                    timestamp={a.timestamp}
                    labelStart={a.labelStart}
                    reply={a.reply}
                    staticMode={staticMode}
                  >
                    {a.tool ? (
                      <ToolCard
                        icon={a.tool.icon}
                        label={a.tool.label}
                        summary={a.tool.summary}
                        start={a.tool.start}
                        doneAt={a.tool.doneAt}
                        staticMode={staticMode}
                      />
                    ) : null}
                  </MessageAgent>
                ))}
              </div>
            </div>
            <Composer staticMode={staticMode} />
          </div>
        </div>
      </ChatFrame>
    </AbsoluteFill>
  );
}

export const STATIC_FALLBACK_FRAME = 425;
