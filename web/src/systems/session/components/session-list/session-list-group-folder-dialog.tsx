import { CreateDestinationPicker } from "@/systems/workspace";

import {
  useSessionListGroupFolderCreate,
  type SessionGroupFolderAxis,
} from "../../hooks/use-session-list-group-folder-create";
import { SessionFolderDialogs } from "./session-folder-dialogs";

type GroupFolderCreate = ReturnType<typeof useSessionListGroupFolderCreate>;

/**
 * New folder for a group's project and profile: the name dialog, with a picker for each axis
 * the opener left open. The group header's `+` and every project menu's New ▸ open it.
 */
export function SessionListGroupFolderDialog({
  folder,
  name,
  testId,
}: {
  folder: GroupFolderCreate;
  /** The settled project's name, shown when only the profile is left to pick. */
  name: string;
  /** `<id>-destination`, `<id>-project`, `<id>-profile`. */
  testId: string;
}) {
  return (
    <SessionFolderDialogs
      state={folder.open ? { kind: "create" } : null}
      onClose={folder.close}
      onCreate={folder.create}
      onRename={() => Promise.resolve()}
      onDelete={() => Promise.resolve()}
      destination={<GroupFolderDestination folder={folder} name={name} testId={testId} />}
    />
  );
}

/** Where a new folder lands, with a picker for each axis the group leaves open. */
function GroupFolderDestination({
  folder,
  name,
  testId,
}: {
  folder: GroupFolderCreate;
  name: string;
  testId: string;
}) {
  if (!folder.project && !folder.profile) return undefined;
  return (
    <p
      className="flex flex-wrap items-baseline gap-x-1 text-small-body text-muted-foreground"
      data-testid={`${testId}-destination`}
    >
      In
      {folder.project ? (
        <FolderAxisPicker axis="Project" testId={`${testId}-project`} {...folder.project} />
      ) : (
        <span className="font-medium text-fg">{name}</span>
      )}
      {folder.profile ? (
        <>
          <span aria-hidden="true">·</span>
          <FolderAxisPicker axis="Profile" testId={`${testId}-profile`} {...folder.profile} />
        </>
      ) : null}
    </p>
  );
}

function FolderAxisPicker({
  axis,
  testId,
  options,
  value,
  onChange,
}: SessionGroupFolderAxis & { axis: string; testId: string }) {
  return (
    <CreateDestinationPicker
      axis={axis}
      options={options}
      value={value}
      onChange={onChange}
      testId={testId}
    />
  );
}
