import { MenubarContent, MenubarMenu, MenubarTrigger } from "@compozy/ui";

import { usePaletteRegistry } from "../../hooks/use-palette-registry";
import { dockApps } from "../../lib/app-registry";
import { MenubarCommandGroups } from "./menubar-command-groups";
import { MenubarCommandItem } from "./menubar-command-item";

export interface GoMenuProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Runs a registry command through the one dispatch seam. */
  onRun: (commandId: string) => void;
}

/**
 * Navigation menu. Groups and order mirror the dock strip exactly, so the two
 * ways to reach a surface teach the same map (recognition over recall). The
 * app rows themselves are `app.open.*` registry commands, so their availability
 * tracks the live client instead of a locally-computed flag.
 */
export function GoMenu({ open, onOpenChange, onRun }: GoMenuProps) {
  const registry = usePaletteRegistry();
  const has = (commandId: string) => registry.byId.has(commandId);
  const dockGroups = dockApps().reduce<ReturnType<typeof dockApps>>((groups, group) => {
    const available = group.filter(app => has(`app.open.${app.id}`));
    if (available.length > 0) groups.push(available);
    return groups;
  }, []);
  return (
    <MenubarMenu open={open} onOpenChange={onOpenChange}>
      <MenubarTrigger>Go</MenubarTrigger>
      <MenubarContent align="start" data-testid="os-menu-go">
        <MenubarCommandGroups
          groups={[
            {
              id: "palette",
              content: has("palette.open") ? (
                <MenubarCommandItem commandId="palette.open" onRun={onRun} />
              ) : null,
            },
            ...dockGroups.map(group => ({
              id: `apps:${group.map(app => app.id).join(",")}`,
              content: (
                <>
                  {group.map(app => (
                    <MenubarCommandItem
                      commandId={`app.open.${app.id}`}
                      key={app.id}
                      onRun={onRun}
                    />
                  ))}
                </>
              ),
            })),
            {
              id: "workspace",
              content: has("workspace.picker") ? (
                <MenubarCommandItem commandId="workspace.picker" onRun={onRun} />
              ) : null,
            },
          ]}
        />
      </MenubarContent>
    </MenubarMenu>
  );
}
