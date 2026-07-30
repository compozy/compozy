const MCP_PACKAGE_PATTERN = /^(?:@[a-z0-9][a-z0-9._-]*\/)?[a-z0-9][a-z0-9._-]*$/;
const DOCKER_IMAGE_PATTERN = /^[a-z0-9][a-z0-9.-]*(?::[0-9]+)?(?:\/[a-z0-9][a-z0-9._-]*)+$/;
const DOCKER_DIGEST_PATTERN = /^sha256:[a-f0-9]{64}$/;
const LOCAL_HOST_SUFFIXES = [".localhost", ".local", ".internal", ".intranet", ".home", ".lan"];

export function isExactMCPPackageName(value: string): boolean {
  return value === value.trim() && MCP_PACKAGE_PATTERN.test(value);
}

export function isExactDockerImageName(value: string): boolean {
  return value === value.trim() && DOCKER_IMAGE_PATTERN.test(value);
}

export function isVerifiedDockerDigest(value: string): boolean {
  return DOCKER_DIGEST_PATTERN.test(value);
}

export function isMCPLaunchArgument(value: string): boolean {
  return value.trim() !== "" && !value.includes("\0");
}

export function isPublicMCPRemoteURL(value: string): boolean {
  let parsed: URL;
  try {
    parsed = new URL(value.trim());
  } catch {
    return false;
  }
  if (
    parsed.protocol !== "https:" ||
    parsed.hostname === "" ||
    parsed.username !== "" ||
    parsed.password !== "" ||
    parsed.hash !== ""
  ) {
    return false;
  }

  const hostname = parsed.hostname
    .toLowerCase()
    .replace(/^\[|\]$/g, "")
    .replace(/\.$/, "");
  const ipv4 = parseIPv4(hostname);
  if (ipv4) return !isBlockedIPv4(ipv4);

  const ipv6 = parseIPv6(hostname);
  if (ipv6) return !isBlockedIPv6(ipv6);

  return hostname.includes(".") && !LOCAL_HOST_SUFFIXES.some(suffix => hostname.endsWith(suffix));
}

export function mcpRemoteURLQueryNames(value: string): Set<string> {
  try {
    return new Set(new URL(value).searchParams.keys());
  } catch {
    return new Set();
  }
}

type IPv4Address = readonly [number, number, number, number];

function parseIPv4(hostname: string): IPv4Address | null {
  const parts = hostname.split(".");
  if (parts.length !== 4) return null;
  const octets = parts.map(part => (/^\d{1,3}$/.test(part) ? Number(part) : Number.NaN));
  if (octets.some(octet => !Number.isInteger(octet) || octet < 0 || octet > 255)) return null;
  return octets as unknown as IPv4Address;
}

function isBlockedIPv4([a, b, c]: IPv4Address): boolean {
  return (
    a === 0 ||
    a === 10 ||
    a === 127 ||
    (a === 100 && b >= 64 && b <= 127) ||
    (a === 169 && b === 254) ||
    (a === 172 && b >= 16 && b <= 31) ||
    (a === 192 && b === 168) ||
    (a === 192 && b === 0 && c === 0) ||
    (a === 192 && b === 0 && c === 2) ||
    (a === 192 && b === 88 && c === 99) ||
    (a === 198 && (b === 18 || b === 19)) ||
    (a === 198 && b === 51 && c === 100) ||
    (a === 203 && b === 0 && c === 113) ||
    a >= 224
  );
}

function parseIPv6(hostname: string): number[] | null {
  if (!hostname.includes(":")) return null;
  const halves = hostname.split("::");
  if (halves.length > 2) return null;

  const left = parseIPv6Words(halves[0]);
  const right = parseIPv6Words(halves[1] ?? "");
  if (!left || !right) return null;
  const missingWords = 8 - left.length - right.length;
  if ((halves.length === 1 && missingWords !== 0) || missingWords < (halves.length === 2 ? 1 : 0)) {
    return null;
  }
  return [...left, ...Array.from({ length: missingWords }, () => 0), ...right];
}

function parseIPv6Words(value: string): number[] | null {
  if (value === "") return [];
  const parts = value.split(":");
  const words: number[] = [];
  for (const [index, part] of parts.entries()) {
    const ipv4 = parseIPv4(part);
    if (ipv4 && index === parts.length - 1) {
      words.push((ipv4[0] << 8) | ipv4[1], (ipv4[2] << 8) | ipv4[3]);
      continue;
    }
    if (!/^[a-f0-9]{1,4}$/i.test(part)) return null;
    words.push(Number.parseInt(part, 16));
  }
  return words;
}

function isBlockedIPv6(words: number[]): boolean {
  const allZero = words.every(word => word === 0);
  const loopback = words.slice(0, 7).every(word => word === 0) && words[7] === 1;
  const mappedIPv4 =
    words.slice(0, 5).every(word => word === 0) && words[5] === 0xffff
      ? ([words[6] >> 8, words[6] & 0xff, words[7] >> 8, words[7] & 0xff] as IPv4Address)
      : null;
  if (mappedIPv4) return isBlockedIPv4(mappedIPv4);

  return (
    allZero ||
    loopback ||
    (words[0] & 0xfe00) === 0xfc00 ||
    (words[0] & 0xffc0) === 0xfe80 ||
    (words[0] & 0xff00) === 0xff00 ||
    (words[0] === 0x64 && words[1] === 0xff9b && words.slice(2, 6).every(word => word === 0)) ||
    (words[0] === 0x64 && words[1] === 0xff9b && words[2] === 1) ||
    (words[0] === 0x100 && words.slice(1, 4).every(word => word === 0)) ||
    (words[0] === 0x2001 && words[1] === 0) ||
    (words[0] === 0x2001 && words[1] === 2 && words[2] === 0) ||
    (words[0] === 0x2001 && words[1] === 0x0db8) ||
    words[0] === 0x2002
  );
}
