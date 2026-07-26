import { Button, Spinner } from "@agh/ui";

export function SessionLoadOlderButton({
  isLoading,
  onClick,
}: {
  isLoading: boolean;
  onClick: () => void;
}) {
  return (
    <div className="flex justify-center py-2">
      <Button
        type="button"
        variant="neutral"
        size="sm"
        disabled={isLoading}
        aria-busy={isLoading}
        onClick={onClick}
        data-testid="load-older-session-messages"
      >
        {isLoading ? <Spinner aria-hidden="true" /> : null}
        {isLoading ? "Loading older messages" : "Load older messages"}
      </Button>
    </div>
  );
}
