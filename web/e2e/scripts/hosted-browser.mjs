import { chromium } from "@playwright/test";
import { createInterface } from "node:readline";
import { TERMINAL_SERVER_OP, TERMINAL_SUBPROTOCOL } from "../../src/generated/terminal-wire";

const edgePort = process.argv[2];
const cloud = "https://cloud.test";
const authorizePath = "/v1/browser/authorize";
const providerPath = "/fixture/google/authorize";
const browser = await chromium.launch({
  args: [
    `--host-resolver-rules=MAP cloud.test 127.0.0.1:${edgePort}, MAP *.compozy.run 127.0.0.1:${edgePort}, MAP * ~NOTFOUND`,
    "--ignore-certificate-errors",
    "--no-proxy-server",
  ],
});
const context = await browser.newContext();
const pages = new Map();

await context.addInitScript(
  ({ opcodes, subprotocol }) => {
    const encode = data =>
      btoa(String.fromCharCode(...new Uint8Array(data)))
        .replaceAll("+", "-")
        .replaceAll("/", "_")
        .replaceAll("=", "");
    const text = value => new TextEncoder().encode(value);
    const keyRequest = (mode, run) =>
      new Promise((resolve, reject) => {
        const open = indexedDB.open("hosted-browser-integration", 1);
        open.onupgradeneeded = () => open.result.createObjectStore("keys");
        open.onerror = () => reject(open.error);
        open.onsuccess = () => {
          const database = open.result;
          const transaction = database.transaction("keys", mode);
          const request = run(transaction.objectStore("keys"));
          transaction.oncomplete = () => {
            database.close();
            resolve(request.result);
          };
          transaction.onerror = () => {
            database.close();
            reject(transaction.error);
          };
        };
      });
    const generate = () =>
      crypto.subtle.generateKey({ name: "ECDSA", namedCurve: "P-256" }, false, ["sign", "verify"]);
    const keyPair = () => keyRequest("readonly", store => store.get("browser"));
    const proof = async (method, pathname, now, wrongKey) => {
      const key = wrongKey ? await generate() : await keyPair();
      const jwk = await crypto.subtle.exportKey("jwk", key.publicKey);
      const header = { typ: "dpop+jwt", alg: "ES256", jwk };
      const target = new URL(pathname, location.origin);
      target.search = "";
      const claims = { htm: method, htu: target.href, iat: now, jti: crypto.randomUUID() };
      const signingInput = `${encode(text(JSON.stringify(header)))}.${encode(text(JSON.stringify(claims)))}`;
      const signed = await crypto.subtle.sign(
        { name: "ECDSA", hash: "SHA-256" },
        key.privateKey,
        text(signingInput)
      );
      return `${signingInput}.${encode(signed)}`;
    };
    const streams = new Map();

    window.hostedFixture = {
      async prepare(replace) {
        let key = await keyPair();
        if (!key || replace) {
          key = await generate();
          await keyRequest("readwrite", store => store.put(key, "browser"));
        }
        if (key.privateKey.extractable) throw new Error("Browser key must be non-extractable");
        const jwk = await crypto.subtle.exportKey("jwk", key.publicKey);
        const verifier = encode(crypto.getRandomValues(new Uint8Array(32)));
        const state = crypto.randomUUID();
        sessionStorage.setItem("hosted-state", state);
        sessionStorage.setItem("hosted-verifier", verifier);
        return {
          hostname: location.hostname,
          public_jwk: encode(text(JSON.stringify(jwk))),
          code_challenge: encode(await crypto.subtle.digest("SHA-256", text(verifier))),
          state,
          provider: "google",
        };
      },
      callback() {
        const query = new URL(location.href).searchParams;
        if (query.get("state") !== sessionStorage.getItem("hosted-state")) {
          throw new Error("Browser OAuth state did not round-trip");
        }
        return {
          code: query.get("code") ?? "",
          error: query.get("error") ?? "",
          verifier: sessionStorage.getItem("hosted-verifier"),
        };
      },
      async request({ method, path, body, now, mode, token }) {
        const headers = { "Content-Type": "application/json" };
        if (mode !== "unsigned")
          headers.DPoP = token ?? (await proof(method, path, now, mode === "wrong"));
        const options = { method, headers };
        if (body != null && method !== "GET" && method !== "HEAD")
          options.body = JSON.stringify(body);
        const response = await fetch(path, options);
        return {
          status: response.status,
          body: response.status === 204 ? null : await response.json(),
          proof: headers.DPoP,
        };
      },
      async eventSource({ name, path }) {
        const source = new EventSource(path);
        const state = { open: false, events: [], closed: null };
        source.addEventListener("open", () => {
          state.open = true;
        });
        source.addEventListener("error", () => {
          state.closed = Date.now();
          source.close();
        });
        return new Promise((resolve, reject) => {
          const timeout = setTimeout(() => reject(new Error("SSE stream failed to open")), 5000);
          source.addEventListener(
            "open",
            () => {
              clearTimeout(timeout);
              resolve(true);
            },
            { once: true }
          );
          source.addEventListener("message", event => state.events.push(event.data));
          for (const type of [
            "transcript_snapshot",
            "transcript_delta",
            "session_stopped",
            "session_catalog_changed",
          ]) {
            source.addEventListener(type, event => state.events.push(event.data));
          }
          streams.set(name, state);
        });
      },
      async terminal({ name, path, input, marker }) {
        const url = new URL(path, location.origin);
        url.protocol = "wss:";
        const socket = new WebSocket(url, subprotocol);
        socket.binaryType = "arraybuffer";
        const state = { open: false, output: "", closed: null, code: null };
        streams.set(name, state);
        socket.addEventListener("close", event => {
          state.closed = Date.now();
          state.code = event.code;
        });
        return new Promise((resolve, reject) => {
          const timeout = setTimeout(
            () => reject(new Error("Terminal output did not arrive")),
            5000
          );
          socket.addEventListener("open", () => {
            state.open = true;
          });
          socket.addEventListener("message", ({ data }) => {
            const frame = new Uint8Array(data);
            if (frame[0] === opcodes.attached)
              socket.send(Uint8Array.from(atob(input), char => char.charCodeAt(0)));
            if (frame[0] !== opcodes.output) return;
            state.output += new TextDecoder().decode(frame.subarray(9));
            if (state.output.includes(marker)) {
              clearTimeout(timeout);
              resolve(true);
            }
          });
        });
      },
      state(name) {
        return streams.get(name);
      },
    };
  },
  { opcodes: TERMINAL_SERVER_OP, subprotocol: TERMINAL_SUBPROTOCOL }
);

async function pageFor(hostname) {
  if (!pages.has(hostname)) {
    const page = await context.newPage();
    const response = await page.goto(`https://${hostname}/`);
    if (response.status() !== 200 || !(await response.text()).includes('id="app"')) {
      throw new Error(`Hosted SPA shell did not load on ${hostname}`);
    }
    pages.set(hostname, page);
  }
  return pages.get(hostname);
}

async function authorize(page, replace) {
  const form = await page.evaluate(replace => window.hostedFixture.prepare(replace), replace);
  let response = await page.goto(`${cloud}${authorizePath}?${new URLSearchParams(form)}`, {
    waitUntil: "domcontentloaded",
  });
  if (page.url().startsWith(`${cloud}${authorizePath}`) && response.status() === 400) {
    const [provider] = await Promise.all([
      page.waitForNavigation({ waitUntil: "domcontentloaded" }),
      page.locator("#provider-form").evaluate(form => {
        form.elements.namedItem("cf-turnstile-response").value = "pass-browser_authorize";
        const provider = document.createElement("input");
        provider.type = "hidden";
        provider.name = "provider";
        provider.value = "google";
        form.append(provider);
        form.submit();
      }),
    ]);
    const callback = provider.headers().location;
    if (!page.url().startsWith(`${cloud}${providerPath}`) || !callback) {
      throw new Error("Fake Google sign-in did not start");
    }
    response = await page.goto(new URL(callback, cloud).href, { waitUntil: "domcontentloaded" });
  }
  if (page.url().startsWith(cloud)) {
    const html = await response.text();
    return {
      status: response.status(),
      error: html.includes("computer_not_owned") ? "computer_not_owned" : "cloud_error",
    };
  }
  const result = await page.evaluate(() => window.hostedFixture.callback());
  return { status: response.status(), ...result };
}

async function execute(command) {
  const { action, hostname, input } = command;
  if (action === "clearBinding") {
    await context.clearCookies({ name: "__Host-compozy_bd" });
    return true;
  }
  if (action === "cookies") return context.cookies();
  const page = await pageFor(hostname);
  switch (action) {
    case "authorize":
      return authorize(page, input?.replace ?? false);
    case "request":
      return page.evaluate(args => window.hostedFixture.request(args), input);
    case "pwa":
      await page.waitForFunction(() => navigator.serviceWorker.controller !== null, null, {
        timeout: 8000,
      });
      return page.evaluate(async () => (await fetch("/manifest.webmanifest")).json());
    case "eventSource":
      return page.evaluate(args => window.hostedFixture.eventSource(args), input);
    case "terminal":
      return page.evaluate(args => window.hostedFixture.terminal(args), input);
    case "streamState":
      return page.evaluate(name => window.hostedFixture.state(name), input);
    default:
      throw new Error(`Unknown hosted browser fixture action ${action}`);
  }
}

try {
  for await (const line of createInterface({ input: process.stdin })) {
    try {
      console.log(JSON.stringify({ value: await execute(JSON.parse(line)) }));
    } catch (error) {
      console.log(JSON.stringify({ error: error.message }));
    }
  }
} finally {
  await browser.close();
}
