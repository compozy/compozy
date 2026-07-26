import { Component, type ErrorInfo, type ReactNode } from "react";
import { AlertTriangle, RefreshCw } from "lucide-react";

import { Button, Empty } from "@agh/ui";

interface OsWindowErrorBoundaryProps {
  children: ReactNode;
  /** Window identity for the alert copy (the app title). */
  title: string;
}

interface OsWindowErrorBoundaryState {
  error: Error | null;
}

/**
 * Per-window render failure isolation (rewrite of the route-level
 * `AppRouteErrorBoundary`): one window crashing never takes down the desktop
 * or its sibling windows. Class component per React's error-boundary contract.
 */
export class OsWindowErrorBoundary extends Component<
  OsWindowErrorBoundaryProps,
  OsWindowErrorBoundaryState
> {
  state: OsWindowErrorBoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): OsWindowErrorBoundaryState {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    if (typeof console !== "undefined" && typeof console.error === "function") {
      console.error("Window render failed", error, info.componentStack);
    }
  }

  private handleRetry = (): void => {
    this.setState({ error: null });
  };

  render(): ReactNode {
    const { error } = this.state;
    if (error === null) return this.props.children;

    return (
      <div
        className="flex min-h-full w-full items-center justify-center p-6"
        role="alert"
        data-testid="os-window-error"
      >
        <Empty
          className="max-w-md"
          description={
            error.message.trim().length > 0
              ? error.message
              : `The ${this.props.title} window could not be rendered.`
          }
          icon={AlertTriangle}
          title={`${this.props.title} failed to render`}
          action={
            <Button onClick={this.handleRetry} size="sm" type="button" variant="outline">
              <RefreshCw className="size-3" />
              Retry
            </Button>
          }
        />
      </div>
    );
  }
}
