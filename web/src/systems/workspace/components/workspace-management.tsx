import { Folder, Archive } from "lucide-react";
import { Alert, AlertDescription, ListGroup, ListingRow, Spinner } from "@compozy/ui";
import { useArchivedWorkspaces, useWorkspaces } from "../hooks/use-workspaces";
import type { WorkspacePayload } from "../types";
import { WorkspaceProjectActions } from "./workspace-project-actions";

function ProjectRows({ projects }: { projects: WorkspacePayload[] }) {
  return projects.map(project => (
    <ListingRow key={project.id} interactive={false}>
      <ListingRow.Icon>{project.archived_at ? <Archive /> : <Folder />}</ListingRow.Icon>
      <ListingRow.Main>
        <ListingRow.Name>{project.name}</ListingRow.Name>
        <ListingRow.Description>{project.root_dir}</ListingRow.Description>
      </ListingRow.Main>
      <ListingRow.Trail>
        <WorkspaceProjectActions workspace={project} />
      </ListingRow.Trail>
    </ListingRow>
  ));
}

export function WorkspaceManagement() {
  const active = useWorkspaces();
  const archived = useArchivedWorkspaces();
  if (active.isPending || archived.isPending) return <Spinner aria-label="Loading projects" />;
  if (active.error || archived.error)
    return (
      <Alert variant="danger">
        <AlertDescription>{active.error?.message ?? archived.error?.message}</AlertDescription>
      </Alert>
    );
  return (
    <div className="space-y-4" data-testid="workspace-management">
      <ListGroup label="Active" count={active.data.length}>
        <ProjectRows projects={active.data} />
      </ListGroup>
      <ListGroup label="Archived" count={archived.data.length}>
        {archived.data.length ? (
          <ProjectRows projects={archived.data} />
        ) : (
          <p className="px-4 py-3 text-muted">No archived projects.</p>
        )}
      </ListGroup>
    </div>
  );
}
