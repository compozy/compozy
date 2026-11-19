import type { ReactNode } from "react";

/** Stand-in browser window so the page reads as the tab the provider redirects to. */
export function BrowserFrame({ children }: { children: ReactNode }) {
  return (
    <div className="cb-stage">
      <div className="cb-browser">
        <div className="cb-browser__bar">
          <div className="cb-browser__lights" aria-hidden="true">
            <span />
            <span />
            <span />
          </div>
          <span className="cb-browser__url">
            http://<b>127.0.0.1:2123</b>/api/mcp/oauth/callback?code=…&amp;state=…
          </span>
        </div>
        {children}
      </div>
    </div>
  );
}
