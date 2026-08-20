import { AlertTriangle, ExternalLink, Globe2 } from "lucide-react";

import { Alert, AlertActions, AlertDescription, AlertTitle, Button } from "@compozy/ui";

import { ShortcutBindingKeys, type WindowManagerSettingsSection } from "@/systems/os";
import { isDesktopShell } from "@/systems/os/lib/desktop-shell-bridge";

import type { GlobalShortcutRecorderModel } from "../../hooks/use-global-shortcut-recorder";
import { WindowManagerBindingConflict } from "./window-manager-binding-conflict";

const ACCESSIBILITY_SETTINGS_URL =
  "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility";

type GlobalRegistration = WindowManagerSettingsSection["globalShortcuts"][number];

function statusText(shell: boolean, status: GlobalRegistration["status"]): string {
  if (!shell) return "desktop only";
  switch (status) {
    case "registered":
      return "active";
    case "failed_in_use":
      return "captured";
    case "failed_permission":
      return "permission required";
    case "unsupported":
      return "unsupported";
  }
}

function reasonText(shell: boolean, registration: GlobalRegistration): string | null {
  if (!shell) return "requires desktop shell";
  if (registration.status === "failed_in_use") {
    return "unavailable — in use by another application";
  }
  return registration.reason;
}

export function WindowManagerGlobalHotkeys({
  recorder,
  section,
}: {
  recorder: GlobalShortcutRecorderModel;
  section: WindowManagerSettingsSection;
}) {
  const shell = isDesktopShell();
  const titleFor = (commandId: string) =>
    section.commands.find(command => command.id === commandId)?.title ?? commandId;
  const needsAccessibility =
    shell && section.globalShortcuts.some(item => item.status === "failed_permission");

  return (
    <section className="border-t border-line-soft" data-testid="window-manager-global-hotkeys">
      <span aria-live="polite" className="sr-only">
        {recorder.announcement}
      </span>
      <div className="flex items-center gap-2 border-b border-line-soft px-4 py-3">
        <Globe2 aria-hidden="true" className="size-4 text-subtle" />
        <h3 className="text-form-label font-medium text-fg">Global hotkeys</h3>
      </div>

      {needsAccessibility ? (
        <Alert className="m-4" variant="warning">
          <AlertTriangle aria-hidden="true" />
          <AlertTitle>Accessibility permission required</AlertTitle>
          <AlertDescription>
            Allow Compozy to register global hotkeys, then return here to retry.
          </AlertDescription>
          <AlertActions>
            <Button
              nativeButton={false}
              render={<a aria-label="Open System Settings" href={ACCESSIBILITY_SETTINGS_URL} />}
              size="sm"
              variant="outline"
            >
              Open System Settings
              <ExternalLink aria-hidden="true" className="size-3.5" />
            </Button>
          </AlertActions>
        </Alert>
      ) : null}

      {recorder.error ? (
        <p className="px-4 py-2 text-form-hint text-danger" role="alert">
          {recorder.error}
        </p>
      ) : null}

      <div className="divide-y divide-line-soft">
        {section.globalShortcuts.map(registration => {
          const shownChord =
            registration.status === "failed_in_use" && registration.activeChord
              ? registration.activeChord
              : registration.intendedChord;
          const conflict =
            recorder.conflict?.commandId === registration.commandId ? recorder.conflict : null;
          const reason = reasonText(shell, registration);
          return (
            <div className="px-4 py-3" key={registration.commandId}>
              <div className="flex flex-wrap items-center gap-3">
                <div className="min-w-48 flex-1">
                  <p className="text-form-label font-medium text-fg">
                    {titleFor(registration.commandId)}
                  </p>
                  <p className="font-mono text-micro text-faint">{registration.commandId}</p>
                </div>
                <Button
                  aria-label={`Record global hotkey for ${titleFor(registration.commandId)}`}
                  className="min-w-32 justify-center"
                  disabled={!shell || recorder.saving}
                  size="sm"
                  type="button"
                  variant="outline"
                  onClick={() => recorder.start(registration.commandId)}
                >
                  {recorder.recording === registration.commandId ? (
                    "Press keys…"
                  ) : (
                    <ShortcutBindingKeys bindings={[shownChord]} compact />
                  )}
                </Button>
                <div className="w-44 text-right">
                  <p className="text-form-hint text-subtle">
                    {statusText(shell, registration.status)}
                  </p>
                  {reason ? <p className="text-micro text-faint">{reason}</p> : null}
                </div>
              </div>
              {conflict ? (
                <WindowManagerBindingConflict
                  claim={conflict.chord}
                  consequence={`Overwriting leaves ${titleFor(conflict.owner)} without a global hotkey.`}
                  ownerTitle={titleFor(conflict.owner)}
                  overwriteLabel="Overwrite"
                  testId={`global-hotkey-conflict-${registration.commandId}`}
                  onCancel={recorder.dismissConflict}
                  onOverwrite={recorder.overwrite}
                />
              ) : null}
            </div>
          );
        })}
      </div>
      <p className="border-t border-line-soft px-4 py-2 text-micro text-faint">
        physical keys — non-QWERTY layouts may differ from the printed legend
      </p>
    </section>
  );
}
