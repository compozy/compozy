import { Button, Icon } from "@compozy/ui";
import { TriangleAlert } from "@compozy/ui/icons";

export function GroupNote({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-6 items-center gap-2 px-3 py-1 text-micro text-subtle">
      {children}
    </div>
  );
}

export function GroupLoadFailure({
  retryTestId,
  onRetry,
}: {
  retryTestId: string;
  onRetry: () => void;
}) {
  return (
    <GroupNote>
      <Icon as={TriangleAlert} size="sm" className="text-warning" />
      Couldn&rsquo;t load sessions
      <Button
        variant="ghost"
        size="sm"
        className="ml-auto"
        data-testid={retryTestId}
        onClick={onRetry}
      >
        Retry
      </Button>
    </GroupNote>
  );
}
