import { Component, type ErrorInfo, type ReactNode } from "react";

import { Button, Eyebrow } from "@agh/ui";

interface SessionThreadErrorBoundaryProps {
  children: ReactNode;
  onRetry?: () => void;
}

interface SessionThreadErrorBoundaryState {
  error: Error | null;
}

/**
 * Themed fallback for assistant-ui / timeline render failures. Matches
 * `ThreadError` visual language (danger Eyebrow + muted detail + outline retry).
 */
export class SessionThreadErrorBoundary extends Component<
  SessionThreadErrorBoundaryProps,
  SessionThreadErrorBoundaryState
> {
  state: SessionThreadErrorBoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): SessionThreadErrorBoundaryState {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    if (typeof console !== "undefined" && typeof console.error === "function") {
      console.error("Session thread render failed", error, info.componentStack);
    }
  }

  private handleRetry = (): void => {
    this.setState({ error: null });
    this.props.onRetry?.();
  };

  render(): ReactNode {
    const { error } = this.state;
    if (error === null) {
      return this.props.children;
    }

    return (
      <div
        className="flex min-h-full w-full min-w-0 items-center justify-center py-12"
        role="alert"
        data-testid="session-thread-error-boundary"
      >
        <div className="max-w-md text-center">
          <Eyebrow className="text-danger">Thread render failed</Eyebrow>
          <p className="mt-2 text-small-body text-muted">
            {error.message.trim().length > 0
              ? error.message
              : "The transcript could not be rendered."}
          </p>
          <Button
            type="button"
            variant="outline"
            className="mt-4"
            onClick={this.handleRetry}
            data-testid="session-thread-error-boundary-retry"
          >
            Retry
          </Button>
        </div>
      </div>
    );
  }
}
