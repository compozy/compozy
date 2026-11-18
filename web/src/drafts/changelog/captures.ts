/** Real product captures for the releases they show. GitHub notes carry no images, so production needs a field for this. */
import agents from "../landing/assets/s-agents.jpg";
import board from "../landing/assets/board.jpg";
import heartbeat from "../landing/assets/s-heartbeat.jpg";
import workspaces from "../landing/assets/s-workspaces.jpg";
import marketplace from "../marketplace/assets/app-marketplace.jpg";

export const CAPTURES: Record<string, { src: string; alt: string }> = {
  "v0.5.5": { src: marketplace, alt: "The Marketplace by category, with one-step Add" },
  "v0.5.1": { src: heartbeat, alt: "An always-on agent's check-in schedule and sources" },
  "v0.5.0": { src: agents, alt: "A grid of agents that keep watch over projects" },
  "v0.4.0": { src: board, alt: "The sessions board, sorted by who owns the next step" },
  "v0.3.0-beta.38": { src: workspaces, alt: "Sessions across every project" },
};
