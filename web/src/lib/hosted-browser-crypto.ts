export type HostedBrowserProvider = "google" | "github";

const DEFAULT_CLOUD_ORIGIN = "https://cloud.compozy.com";

const IDB_NAME = "compozy-hosted-browser";
const IDB_VERSION = 1;
const IDB_STORE = "keys";
const IDB_KEY_RECORD = "browser-dpop-p256";

export interface BrowserPublicJwk {
  crv: "P-256";
  kty: "EC";
  x: string;
  y: string;
}

export interface BrowserKeyStore {
  get: () => Promise<CryptoKeyPair | null>;
  set: (pair: CryptoKeyPair) => Promise<void>;
  delete: () => Promise<void>;
}

let cloudOriginOverride: string | undefined;
let customKeyStore: BrowserKeyStore | null = null;
let memoryKeyPair: CryptoKeyPair | null = null;
let cachedKeyPair: CryptoKeyPair | null = null;
let inflightKeyPair: Promise<CryptoKeyPair> | null = null;

/** Overrides the cloud origin for authorize and billing links (tests only). */
export function setHostedCloudOrigin(origin: string | undefined): void {
  cloudOriginOverride = origin;
}

/** Returns the Compozy Cloud origin used for browser sign-in and billing links. */
export function hostedCloudOrigin(): string {
  return cloudOriginOverride ?? DEFAULT_CLOUD_ORIGIN;
}

/** Replaces the browser key persistence store (tests only). */
export function setBrowserKeyStore(store: BrowserKeyStore | null): void {
  customKeyStore = store;
  cachedKeyPair = null;
  inflightKeyPair = null;
}

/** Resets in-memory browser crypto state (tests only). */
export function resetHostedBrowserCrypto(): void {
  cloudOriginOverride = undefined;
  customKeyStore = null;
  memoryKeyPair = null;
  cachedKeyPair = null;
  inflightKeyPair = null;
}

function bytesToBase64Url(bytes: Uint8Array): string {
  let binary = "";
  for (let index = 0; index < bytes.length; index++) {
    binary += String.fromCharCode(bytes[index] ?? 0);
  }
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
}

function utf8ToBase64Url(value: string): string {
  return bytesToBase64Url(new TextEncoder().encode(value));
}

function randomBase64Url(byteLength: number): string {
  const bytes = new Uint8Array(byteLength);
  globalThis.crypto.getRandomValues(bytes);
  return bytesToBase64Url(bytes);
}

function randomJti(): string {
  if (typeof globalThis.crypto?.randomUUID === "function") {
    return globalThis.crypto.randomUUID();
  }
  return randomBase64Url(16);
}

const memoryFallbackStore: BrowserKeyStore = {
  async get() {
    return memoryKeyPair;
  },
  async set(pair) {
    memoryKeyPair = pair;
  },
  async delete() {
    memoryKeyPair = null;
  },
};

function openKeyDatabase(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(IDB_NAME, IDB_VERSION);
    request.onupgradeneeded = () => {
      const db = request.result;
      if (!db.objectStoreNames.contains(IDB_STORE)) {
        db.createObjectStore(IDB_STORE);
      }
    };
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error ?? new Error("IndexedDB open failed"));
  });
}

const indexedDbKeyStore: BrowserKeyStore = {
  async get() {
    if (typeof indexedDB === "undefined") return memoryFallbackStore.get();
    try {
      const db = await openKeyDatabase();
      try {
        return await new Promise<CryptoKeyPair | null>((resolve, reject) => {
          const tx = db.transaction(IDB_STORE, "readonly");
          const req = tx.objectStore(IDB_STORE).get(IDB_KEY_RECORD);
          req.onsuccess = () => {
            const value = req.result as CryptoKeyPair | undefined;
            resolve(value?.privateKey && value?.publicKey ? value : null);
          };
          req.onerror = () => reject(req.error ?? new Error("IndexedDB read failed"));
        });
      } finally {
        db.close();
      }
    } catch {
      return memoryFallbackStore.get();
    }
  },
  async set(pair) {
    if (typeof indexedDB === "undefined") {
      await memoryFallbackStore.set(pair);
      return;
    }
    try {
      const db = await openKeyDatabase();
      try {
        await new Promise<void>((resolve, reject) => {
          const tx = db.transaction(IDB_STORE, "readwrite");
          const req = tx.objectStore(IDB_STORE).put(pair, IDB_KEY_RECORD);
          req.onsuccess = () => resolve();
          req.onerror = () => reject(req.error ?? new Error("IndexedDB write failed"));
        });
      } finally {
        db.close();
      }
    } catch {
      await memoryFallbackStore.set(pair);
    }
  },
  async delete() {
    if (typeof indexedDB === "undefined") {
      await memoryFallbackStore.delete();
      return;
    }
    try {
      const db = await openKeyDatabase();
      try {
        await new Promise<void>((resolve, reject) => {
          const tx = db.transaction(IDB_STORE, "readwrite");
          const req = tx.objectStore(IDB_STORE).delete(IDB_KEY_RECORD);
          req.onsuccess = () => resolve();
          req.onerror = () => reject(req.error ?? new Error("IndexedDB delete failed"));
        });
      } finally {
        db.close();
      }
    } catch {
      await memoryFallbackStore.delete();
    }
  },
};

function activeKeyStore(): BrowserKeyStore {
  return customKeyStore ?? indexedDbKeyStore;
}

async function generateBrowserKeyPair(): Promise<CryptoKeyPair> {
  return globalThis.crypto.subtle.generateKey({ name: "ECDSA", namedCurve: "P-256" }, false, [
    "sign",
    "verify",
  ]);
}

/**
 * Loads the persisted non-extractable P-256 browser keypair from IndexedDB,
 * generating and persisting a fresh one when absent.
 */
export async function ensureBrowserKeyPair(): Promise<CryptoKeyPair> {
  if (cachedKeyPair) return cachedKeyPair;
  if (inflightKeyPair) return inflightKeyPair;

  const task = (async () => {
    const store = activeKeyStore();
    const existing = await store.get();
    if (existing?.privateKey && existing?.publicKey) {
      cachedKeyPair = existing;
      return existing;
    }
    const generated = await generateBrowserKeyPair();
    await store.set(generated);
    cachedKeyPair = generated;
    return generated;
  })();

  inflightKeyPair = task;
  try {
    return await task;
  } finally {
    if (inflightKeyPair === task) inflightKeyPair = null;
  }
}

/** Generates a brand-new non-extractable P-256 keypair and overwrites the stored key. */
export async function replaceBrowserKeyPair(): Promise<CryptoKeyPair> {
  cachedKeyPair = null;
  inflightKeyPair = null;
  const store = activeKeyStore();
  await store.delete();
  const generated = await generateBrowserKeyPair();
  await store.set(generated);
  cachedKeyPair = generated;
  return generated;
}

/** Exports the canonical public P-256 JWK fields (`crv`, `kty`, `x`, `y`). */
export async function exportPublicJwk(publicKey: CryptoKey): Promise<BrowserPublicJwk> {
  const jwk = await globalThis.crypto.subtle.exportKey("jwk", publicKey);
  if (
    jwk.kty !== "EC" ||
    jwk.crv !== "P-256" ||
    typeof jwk.x !== "string" ||
    typeof jwk.y !== "string"
  ) {
    throw new Error("Browser key is not an EC P-256 public key.");
  }
  return { crv: "P-256", kty: "EC", x: jwk.x, y: jwk.y };
}

/** Encodes a public JWK as base64url UTF-8 JSON for `/v1/browser/authorize`. */
export function encodePublicJwkParam(jwk: BrowserPublicJwk): string {
  return utf8ToBase64Url(JSON.stringify({ crv: jwk.crv, kty: jwk.kty, x: jwk.x, y: jwk.y }));
}

/** Computes the RFC 7638 SHA-256 JWK thumbprint (base64url, no padding). */
export async function computeJwkThumbprint(jwk: BrowserPublicJwk): Promise<string> {
  const canonical = JSON.stringify({ crv: jwk.crv, kty: jwk.kty, x: jwk.x, y: jwk.y });
  const digest = await globalThis.crypto.subtle.digest(
    "SHA-256",
    new TextEncoder().encode(canonical)
  );
  return bytesToBase64Url(new Uint8Array(digest));
}

/** Generates a 32-byte base64url PKCE `code_verifier` (43 chars). */
export function generateCodeVerifier(): string {
  return randomBase64Url(32);
}

/** Derives the S256 PKCE `code_challenge` from `code_verifier`. */
export async function deriveCodeChallenge(codeVerifier: string): Promise<string> {
  const digest = await globalThis.crypto.subtle.digest(
    "SHA-256",
    new TextEncoder().encode(codeVerifier)
  );
  return bytesToBase64Url(new Uint8Array(digest));
}

/** Generates a random opaque `state` parameter for `/v1/browser/authorize`. */
export function generateAuthState(): string {
  return randomBase64Url(24);
}

/** Builds the Compozy Cloud browser authorization URL (`GET /v1/browser/authorize`). */
export function buildBrowserAuthorizeUrl(params: {
  hostname: string;
  publicJwk: BrowserPublicJwk;
  codeChallenge: string;
  state: string;
  provider?: HostedBrowserProvider;
  cloudOrigin?: string;
}): string {
  const url = new URL("/v1/browser/authorize", params.cloudOrigin ?? hostedCloudOrigin());
  url.searchParams.set("hostname", params.hostname);
  url.searchParams.set("public_jwk", encodePublicJwkParam(params.publicJwk));
  url.searchParams.set("code_challenge", params.codeChallenge);
  url.searchParams.set("code_challenge_method", "S256");
  url.searchParams.set("state", params.state);
  if (params.provider) {
    url.searchParams.set("provider", params.provider);
  }
  return url.toString();
}

/** Builds the Upgrade to Pro link (`https://cloud.compozy.com/billing/upgrade?hostname=...`). */
export function buildUpgradeProUrl(hostname: string, cloudOrigin?: string): string {
  const url = new URL("/billing/upgrade", cloudOrigin ?? hostedCloudOrigin());
  url.searchParams.set("hostname", hostname);
  return url.toString();
}

/** Normalizes a URL into RFC 9449 `htu` (scheme + authority + path, without query or hash). */
export function normalizeDpopHtu(url: string, baseOrigin?: string): string {
  const resolvedBase =
    baseOrigin ?? (typeof window !== "undefined" ? window.location.origin : "http://localhost");
  const parsed = new URL(url, resolvedBase);
  return `${parsed.origin}${parsed.pathname}`;
}

/** Signs a fresh RFC 9449 browser DPoP proof (`typ: "dpop+jwt"`, `alg: "ES256"`, no `ath`). */
export async function buildBrowserDPoP(
  method: string,
  url: string,
  options: { keyPair?: CryptoKeyPair; baseOrigin?: string; nowSeconds?: number } = {}
): Promise<string> {
  const keyPair = options.keyPair ?? (await ensureBrowserKeyPair());
  const publicJwk = await exportPublicJwk(keyPair.publicKey);
  const header = { typ: "dpop+jwt", alg: "ES256", jwk: publicJwk };
  const payload = {
    htm: method.toUpperCase(),
    htu: normalizeDpopHtu(url, options.baseOrigin),
    iat: options.nowSeconds ?? Math.floor(Date.now() / 1000),
    jti: randomJti(),
  };
  const signingInput = `${utf8ToBase64Url(JSON.stringify(header))}.${utf8ToBase64Url(JSON.stringify(payload))}`;
  const signature = await globalThis.crypto.subtle.sign(
    { name: "ECDSA", hash: "SHA-256" },
    keyPair.privateKey,
    new TextEncoder().encode(signingInput)
  );
  return `${signingInput}.${bytesToBase64Url(new Uint8Array(signature))}`;
}
