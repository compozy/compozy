const origin = process.env.CLOUD_ORIGIN;
const revision = process.env.CLOUD_REVISION;
const rollout = `${revision}.${process.env.GITHUB_RUN_ID}.${process.env.GITHUB_RUN_ATTEMPT}`;
if (!origin || !revision) throw new Error("CLOUD_ORIGIN and CLOUD_REVISION are required");

async function request(path: string, init: RequestInit = {}) {
  const response = await fetch(new URL(path, origin), {
    ...init,
    redirect: "error",
    signal: AbortSignal.timeout(10_000),
  });
  if (!response.ok) throw new Error(`Cloud smoke check returned HTTP ${response.status}`);
  return response;
}

async function waitForRevision() {
  for (let attempt = 0; attempt < 20; attempt++) {
    const health = await request("/healthz").catch(() => undefined);
    if (health) {
      const body: { status?: string } = await health.json();
      if (body.status === "ok" && health.headers.get("X-Compozy-Cloud-Revision") === rollout) {
        return;
      }
    }
    await Bun.sleep(3_000);
  }
  throw new Error("Cloud readiness did not report the deployed revision");
}

await waitForRevision();

const jwks: { keys?: { kid?: string; kty?: string; crv?: string; x?: string }[] } = await (
  await request("/.well-known/jwks.json")
).json();
if (!jwks.keys?.some(key => key.kty === "OKP" && key.crv === "Ed25519" && key.kid && key.x)) {
  throw new Error("Cloud JWKS has no usable Ed25519 verification key");
}

const flow: { device_code?: string; user_code?: string; verification_uri?: string } = await (
  await request("/v1/device-flow", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      provider: "github",
      client_name: "Cloud deployment smoke",
      client_platform: "linux",
      installation_id: crypto.randomUUID(),
    }),
  })
).json();
if (!flow.device_code) throw new Error("Cloud device flow did not issue a code");
await request(`/v1/device-flow/${encodeURIComponent(flow.device_code)}`, { method: "DELETE" });
if (
  !flow.user_code ||
  new URL(flow.verification_uri ?? "/", origin).href !== new URL("/activate", origin).href
) {
  throw new Error("Cloud device flow returned invalid activation details");
}
console.log("Cloud readiness, JWKS and device-flow start and cancellation passed.");
