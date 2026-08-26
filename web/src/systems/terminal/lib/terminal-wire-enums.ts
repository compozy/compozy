import { z } from "zod";

export const terminalActorKindSchema = z.enum(["human", "agent", "system"]);
export const terminalLeaseStateSchema = z.enum(["agent_owned", "human_owned", "available"]);
export const terminalModeSchema = z.enum(["pty", "pipe"]);
