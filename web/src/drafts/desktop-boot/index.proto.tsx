import "@/drafts/desktop-boot/boot.css";
import "@/drafts/desktop-boot/boot-panel.css";
import "@/drafts/desktop-boot/boot-runner.css";

import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { BootWindow } from "@/drafts/desktop-boot/boot-window";
import { SCENES } from "@/drafts/desktop-boot/scenes";

export const meta = definePrototype({
  title: "Desktop boot and install window",
  surface: "desktop/pages/boot.html",
  states: [
    { id: "resolving", label: "Resolving" },
    { id: "download", label: "First run: download" },
    { id: "verify", label: "First run: verify" },
    { id: "install", label: "First run: install" },
    { id: "migrate", label: "Update: upgrading data" },
    { id: "migrate-slow", label: "Update: long data upgrade" },
    { id: "stalled", label: "Update: no progress" },
    { id: "starting", label: "Update: starting" },
    { id: "starting-retry", label: "Starting, attempt 2" },
    { id: "attaching", label: "Attaching" },
    { id: "ready", label: "Ready" },
    { id: "updating", label: "Updating from the app" },
    { id: "disconnected", label: "Disconnected" },
    { id: "skew", label: "Version skew" },
    { id: "error", label: "Error" },
    { id: "error-details", label: "Error, technical details" },
  ],
});

export default function Prototype({ state }: PrototypeProps) {
  return (
    <div className="boot-stage">
      <div className="boot-frame">
        <div className="boot-frame__lights" aria-hidden="true">
          <span />
          <span />
          <span />
        </div>
        <BootWindow key={state} scene={SCENES[state] ?? SCENES.resolving} />
      </div>
    </div>
  );
}
