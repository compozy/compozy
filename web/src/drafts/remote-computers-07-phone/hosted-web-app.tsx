import { toast } from "@compozy/ui";

import { HostedComputerMenu } from "@/systems/computers/components/hosted-computer-menu";
import type {
  ComputerFixture,
  HostedPeerFixture,
  RemoteScope,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import {
  PHONE_FULL_SCOPES,
  PHONE_LIVE_TRANSCRIPT,
  PHONE_PEERS_MULTI,
  PHONE_STUDIO_SESSIONS,
} from "@/systems/computers/mocks/remote-computers-fixtures-phone";
import type {
  RemoteSessionRowEntry,
  TranscriptTurnFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures-sessions";

import { RemoteSessionsWindow } from "../remote-computers-shared-sessions/remote-sessions-window";

function minutesFromLabel(label: string): number {
  const value = Number.parseInt(label, 10);
  if (Number.isNaN(value)) return 0;
  if (label.endsWith("h")) return value * 60;
  if (label.endsWith("d")) return value * 60 * 24;
  return value;
}

/**
 * The tab is served by the computer itself, so its sessions are "this computer" to the web app:
 * no per-row computer chips, and nothing from other computers (US-022 AC-4).
 */
function hostedSessions(computer: ComputerFixture): RemoteSessionRowEntry[] {
  return PHONE_STUDIO_SESSIONS.map(session => ({
    ...session,
    computerId: "local",
    isLocal: true,
    iconKind: computer.iconKind,
    platform: computer.platform,
    provider: "claude",
    needsYouPhrase: session.state === "attention" ? "Waiting for approval" : undefined,
    minutesAgo: minutesFromLabel(session.updatedLabel),
  }));
}

function hostedTranscript(waitingOnApproval: boolean): TranscriptTurnFixture[] {
  const [request, tool, reply, marker] = PHONE_LIVE_TRANSCRIPT;
  const turns: TranscriptTurnFixture[] = [
    { id: request!.id, role: "user", text: request!.text ?? "" },
    {
      id: reply!.id,
      role: "agent",
      text: reply!.text ?? "",
      toolLine: tool?.preview ? `Ran \`${tool.preview}\`` : undefined,
    },
  ];
  if (waitingOnApproval && marker) {
    turns.push({
      id: marker.id,
      role: "agent",
      text: `Waiting for your approval to run \`${marker.command ?? ""}\`.`,
    });
  }
  return turns;
}

export interface HostedWebAppProps {
  /** The computer serving this tab (`https://<slug>.compozy.run`). */
  computer: ComputerFixture;
  peers?: readonly HostedPeerFixture[];
  scopes?: readonly RemoteScope[];
  /** The open session streams (the agent is working) instead of waiting on an approval. */
  working?: boolean;
  computerMenuOpen?: boolean;
  /** The tab lost its connection to `computer` and is reconnecting. */
  connectionLost?: boolean;
}

/**
 * CompozyOS web app on a computer's hosted address, at browser widths of 1024 px and up (U8):
 * the same shell as the desktop app (system bar, rail, Sessions window), with the header
 * computer picker that lists the account's other hosted routes and **Sign out**.
 */
export function HostedWebApp({
  computer,
  peers = PHONE_PEERS_MULTI,
  scopes = PHONE_FULL_SCOPES,
  working = false,
  computerMenuOpen = false,
  connectionLost = false,
}: HostedWebAppProps) {
  const sessions = hostedSessions(computer);
  const activeSession = sessions.find(item => item.state === "attention") ?? sessions[0]!;
  const session = working ? { ...activeSession, state: "running" as const } : activeSession;

  return (
    <RemoteSessionsWindow
      activeComputer={null}
      activeSession={session}
      canOperateTerminal={scopes.includes("terminal:operate")}
      className="h-auto min-h-0 flex-1"
      hostConnectionLost={connectionLost}
      hostName={computer.name}
      isWorking={working}
      onNewSession={() => toast.message(`New session on ${computer.name}`)}
      scope="api"
      sessions={sessions}
      topbarContext={
        <HostedComputerMenu
          computer={computer}
          defaultOpen={computerMenuOpen}
          onSelectPeer={peer => toast.message(`Opening https://${peer.hostname}`)}
          onSignOut={() =>
            toast.message(
              `Signed out on ${computer.name}. This device stays registered on your account.`
            )
          }
          peers={peers}
          reconnecting={connectionLost}
        />
      }
      transcript={hostedTranscript(!working && !connectionLost)}
    />
  );
}
