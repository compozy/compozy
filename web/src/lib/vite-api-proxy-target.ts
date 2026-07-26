export const defaultApiProxyTarget = "http://localhost:2123/";

const apiProxyTargetEnvKey = "AGH_WEB_API_PROXY_TARGET";

export function resolveApiProxyTarget(env: Record<string, string | undefined>): string {
  return parseApiProxyTarget(env).toString();
}

export function resolveApiProxyOrigin(env: Record<string, string | undefined>): string {
  return parseApiProxyTarget(env).origin;
}

function parseApiProxyTarget(env: Record<string, string | undefined>): URL {
  const rawOverride = env[apiProxyTargetEnvKey];
  const override = rawOverride?.trim();
  if (!override) {
    return new URL(defaultApiProxyTarget);
  }

  let parsed: URL;
  try {
    parsed = new URL(override);
  } catch {
    throw new Error(
      `web: ${apiProxyTargetEnvKey} must be an absolute URL, received ${JSON.stringify(rawOverride)}`
    );
  }

  return parsed;
}
