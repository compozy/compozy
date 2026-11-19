import type { CSSProperties } from "react";

import { Logo } from "@compozy/ui";

// Brand Atmosphere visuals; the production page would ship its own self-hosted copies.
import converge from "../../../../packages/brand/public/brand/visuals/converge-1280.jpg";
import horizon from "../../../../packages/brand/public/brand/visuals/horizon-1280.jpg";
import type { CallbackOutcome, CallbackScene } from "@/drafts/mcp-oauth-callback/scenes";

/** Points on each side of the handshake mark. */
const SPAN = 9;

const STATUS_WORD: Record<CallbackOutcome, string> = {
  connected: "Connected",
  declined: "Not connected",
  expired: "Link expired",
  changed: "Not connected",
  unavailable: "Not connected",
  failed: "Failed",
};

const TONE: Record<CallbackOutcome, "success" | "warning" | "danger"> = {
  connected: "success",
  declined: "warning",
  expired: "warning",
  changed: "warning",
  unavailable: "danger",
  failed: "danger",
};

function Mark({ outcome }: { outcome: CallbackOutcome }) {
  if (outcome === "connected") {
    return (
      <svg viewBox="0 0 16 16" fill="none" aria-hidden="true">
        <path
          d="M4 8.4l2.6 2.6L12 5.4"
          stroke="currentColor"
          strokeWidth="1.8"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
    );
  }
  if (outcome === "expired") {
    return (
      <svg viewBox="0 0 16 16" fill="none" aria-hidden="true">
        <circle cx="8" cy="8" r="5" stroke="currentColor" strokeWidth="1.6" />
        <path d="M8 5.4V8l1.8 1.2" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
      </svg>
    );
  }
  return (
    <svg viewBox="0 0 16 16" fill="none" aria-hidden="true">
      <path
        d="M5.2 5.2l5.6 5.6M10.8 5.2l-5.6 5.6"
        stroke="currentColor"
        strokeWidth="1.8"
        strokeLinecap="round"
      />
    </svg>
  );
}

/**
 * The handshake: CompozyOS on the left, the server on the right, one row of brand points between
 * them. Connected fills the whole row; a failure fills up to the mark and leaves the server side
 * unlit; an unknown server is an empty outline.
 */
function Bridge({ scene }: { scene: CallbackScene }) {
  const connected = scene.outcome === "connected";
  const reached = scene.outcome === "expired" || scene.outcome === "unavailable" ? 0 : SPAN;
  return (
    <div className="cb__bridge" aria-hidden="true">
      {connected ? <img className="cb__stream" src={converge} alt="" /> : null}
      <span className="cb__node cb__node--self">
        <Logo variant="symbol" decorative className="cb__symbol" />
      </span>
      <span className="cb__span">
        {Array.from({ length: SPAN }, (_, index) => (
          <span
            key={index}
            className="cb__point"
            data-on={index < reached ? "lit" : undefined}
            style={{ "--i": index } as CSSProperties}
          />
        ))}
      </span>
      <span className="cb__mark" style={{ "--i": SPAN } as CSSProperties}>
        <Mark outcome={scene.outcome} />
      </span>
      <span className="cb__span" data-side="server">
        {Array.from({ length: SPAN }, (_, index) => (
          <span
            key={index}
            className="cb__point"
            data-on={connected ? "lit" : undefined}
            style={{ "--i": SPAN + 1 + index } as CSSProperties}
          />
        ))}
      </span>
      <span className="cb__node cb__node--server" data-unknown={scene.server ? undefined : true}>
        {scene.server ? scene.server.name.slice(0, 1).toUpperCase() : "?"}
      </span>
    </div>
  );
}

function Facts({ scene }: { scene: CallbackScene }) {
  if (scene.outcome !== "connected" || !scene.server) return null;
  return (
    <dl className="cb__facts">
      <div>
        <dt>Server</dt>
        <dd>
          <span className="cb__fact-strong">{scene.server.name}</span>
          <span className="cb__fact-mono">{scene.server.host}</span>
        </dd>
      </div>
      {scene.availability ? (
        <div>
          <dt>Available in</dt>
          <dd>{scene.availability}</dd>
        </div>
      ) : null}
      {scene.scopes?.length ? (
        <div>
          <dt>Access</dt>
          <dd className="cb__scopes">
            {scene.scopes.map(scope => (
              <span key={scope} className="cb__scope">
                {scope}
              </span>
            ))}
          </dd>
        </div>
      ) : null}
      {scene.renewal ? (
        <div>
          <dt>Sign-in</dt>
          <dd>{scene.renewal}</dd>
        </div>
      ) : null}
    </dl>
  );
}

/** What `GET /api/mcp/oauth/callback` renders for one outcome. */
export function CallbackPage({ scene }: { scene: CallbackScene }) {
  const connected = scene.outcome === "connected";
  return (
    <main className="cb" data-outcome={scene.outcome} data-tone={TONE[scene.outcome]}>
      {connected ? null : <img className="cb__atmosphere" src={horizon} alt="" />}
      <div className="cb__column">
        <Bridge scene={scene} />
        <p className="cb__eyebrow">
          {scene.server ? `${scene.server.name} · MCP server` : "MCP server sign-in"}
        </p>
        <h1 className="cb__title">{scene.title}</h1>
        <p className="cb__detail" role={connected ? undefined : "alert"}>
          {scene.detail}
        </p>
        {scene.code ? <p className="cb__code">{scene.code}</p> : null}
        <Facts scene={scene} />
        {scene.diagnostic ? (
          <details className="cb__diagnostic">
            <summary>Technical details</summary>
            <pre>{scene.diagnostic}</pre>
          </details>
        ) : null}
        <div className="cb__actions">
          <a className="cb__btn" data-variant="primary" href="#open">
            Open CompozyOS
          </a>
          {scene.diagnostic ? (
            <button className="cb__btn" type="button">
              Copy details
            </button>
          ) : null}
        </div>
      </div>
      <p className="cb__status">
        <span className="cb__status-mark" aria-hidden="true" />
        <span className="cb__status-word" role="status">
          {STATUS_WORD[scene.outcome]}
        </span>
        <span>{scene.at}</span>
        <span className="cb__status-hint">
          {connected ? "This tab can be closed" : "Nothing was saved"}
        </span>
      </p>
    </main>
  );
}
