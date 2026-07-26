#!/bin/sh
set -eu

RELEASE_REPO="compozy/agh"
COSIGN_VERSION="v2.2.4"
COSIGN_BASE_URL="https://github.com/sigstore/cosign/releases/download/${COSIGN_VERSION}"
COSIGN_CERT_IDENTITY_REGEXP='^https://github\.com/compozy/agh/\.github/workflows/release\.yml@refs/heads/main$'
COSIGN_CERT_OIDC_ISSUER="https://token.actions.githubusercontent.com"
VERSION="${AGH_VERSION:-latest}"
INSTALL_DIR="${AGH_INSTALL_DIR:-}"
SKIP_BOOTSTRAP="false"
DRY_RUN="false"
COSIGN_BIN=""
COSIGN_NAME=""
COSIGN_SHA256=""

if [ "${AGH_SKIP_BOOTSTRAP:-}" != "" ] && [ "${AGH_SKIP_BOOTSTRAP:-}" != "0" ]; then
  SKIP_BOOTSTRAP="true"
fi

usage() {
  cat <<'USAGE'
AGH installer

Usage:
  curl -fsSL https://agh.network/install.sh | sh
  curl -fsSL https://agh.network/install.sh | sh -s -- [options]

Options:
  --version vX.Y.Z      Install a specific release tag instead of latest.
  --dir PATH            Install agh into PATH.
  --skip-bootstrap      Install the binary only; do not run agh install.
  --dry-run             Print the resolved install plan without writing files.
  -h, --help            Show this help.

Environment:
  AGH_VERSION           Same as --version.
  AGH_INSTALL_DIR       Same as --dir.
  AGH_SKIP_BOOTSTRAP=1  Same as --skip-bootstrap.

Requires:
  curl, tar, and sha256sum or shasum.
  Uses local cosign when available; otherwise downloads a pinned temporary cosign verifier.
USAGE
}

log() {
  printf '%s\n' "$*"
}

fail() {
  printf 'agh installer: %s\n' "$*" >&2
  exit 1
}

resolve_latest_release_tag() {
  resolved_url="$(
    curl -fsSL -o /dev/null -w '%{url_effective}' \
      "https://github.com/${RELEASE_REPO}/releases/latest"
  )" || fail "failed to resolve latest release"
  resolved_tag="${resolved_url##*/}"
  case "$resolved_tag" in
    v[0-9][A-Za-z0-9._-]*)
      printf '%s\n' "$resolved_tag"
      ;;
    *)
      fail "latest release resolved to unexpected ref: ${resolved_url}"
      ;;
  esac
}

verify_file_sha256() {
  file_path="$1"
  expected_sha="$2"
  label="$3"

  if [ "$CHECKSUM_CMD" = "sha256sum" ]; then
    actual_sha="$(sha256sum "$file_path" | awk '{ print $1 }')"
  else
    actual_sha="$(shasum -a 256 "$file_path" | awk '{ print $1 }')"
  fi

  [ "$actual_sha" = "$expected_sha" ] || fail "${label} checksum mismatch"
}

resolve_cosign() {
  if command -v cosign >/dev/null 2>&1; then
    COSIGN_BIN="$(command -v cosign)"
    log "using cosign at ${COSIGN_BIN}"
    return
  fi

  COSIGN_PATH="${TMP_DIR}/${COSIGN_NAME}"
  COSIGN_URL="${COSIGN_BASE_URL}/${COSIGN_NAME}"
  log "downloading pinned cosign verifier ${COSIGN_VERSION}"
  curl -fsSL "$COSIGN_URL" -o "$COSIGN_PATH"
  verify_file_sha256 "$COSIGN_PATH" "$COSIGN_SHA256" "cosign verifier"
  chmod 0755 "$COSIGN_PATH"
  COSIGN_BIN="$COSIGN_PATH"
}

need_arg() {
  if [ "$#" -lt 2 ] || [ "$2" = "" ]; then
    fail "$1 requires a value"
  fi
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version)
      need_arg "$1" "${2:-}"
      VERSION="$2"
      shift 2
      ;;
    --version=*)
      VERSION="${1#*=}"
      [ "$VERSION" != "" ] || fail "--version requires a value"
      shift
      ;;
    --dir)
      need_arg "$1" "${2:-}"
      INSTALL_DIR="$2"
      shift 2
      ;;
    --dir=*)
      INSTALL_DIR="${1#*=}"
      [ "$INSTALL_DIR" != "" ] || fail "--dir requires a value"
      shift
      ;;
    --skip-bootstrap)
      SKIP_BOOTSTRAP="true"
      shift
      ;;
    --dry-run)
      DRY_RUN="true"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "unknown option: $1"
      ;;
  esac
done

case "$(uname -s)" in
  Linux)
    OS="linux"
    ;;
  Darwin)
    OS="darwin"
    ;;
  *)
    fail "unsupported operating system: $(uname -s). AGH installer supports macOS and Linux."
    ;;
esac

case "$(uname -m)" in
  x86_64|amd64)
    ARCH="x86_64"
    ;;
  arm64|aarch64)
    ARCH="arm64"
    ;;
  *)
    fail "unsupported architecture: $(uname -m). AGH installer supports x86_64 and arm64."
    ;;
esac

case "${OS}/${ARCH}" in
  darwin/x86_64)
    COSIGN_NAME="cosign-darwin-amd64"
    COSIGN_SHA256="0e5a77a86115e4c00ba4243db01abceacb13cc06981c45e53ee71f2e1db8ce25"
    ;;
  darwin/arm64)
    COSIGN_NAME="cosign-darwin-arm64"
    COSIGN_SHA256="fcd310e64ecddc1eaa13fe814ac1c9fc02f6f9eacd9a58480ab8160eb8ca381e"
    ;;
  linux/x86_64)
    COSIGN_NAME="cosign-linux-amd64"
    COSIGN_SHA256="97a6a1e15668a75fc4ff7a4dc4cb2f098f929cbea2f12faa9de31db6b42b17d7"
    ;;
  linux/arm64)
    COSIGN_NAME="cosign-linux-arm64"
    COSIGN_SHA256="658087351e1d4f9c396b5f59ee5437461c06128f4ce80ba899ccaa1c0b6a8a62"
    ;;
  *)
    fail "unsupported cosign verifier platform: ${OS}/${ARCH}"
    ;;
esac

ARCHIVE_NAME="agh_${OS}_${ARCH}.tar.gz"

if [ "$VERSION" = "latest" ]; then
  BASE_URL="https://github.com/${RELEASE_REPO}/releases/latest/download"
else
  BASE_URL="https://github.com/${RELEASE_REPO}/releases/download/${VERSION}"
fi

ARCHIVE_URL="${BASE_URL}/${ARCHIVE_NAME}"
CHECKSUM_URL="${BASE_URL}/checksums.txt"
BUNDLE_URL="${BASE_URL}/checksums.txt.sigstore.json"

if [ "$INSTALL_DIR" = "" ]; then
  if [ -d "/usr/local/bin" ] && [ -w "/usr/local/bin" ]; then
    INSTALL_DIR="/usr/local/bin"
  else
    INSTALL_DIR="${HOME}/.local/bin"
  fi
fi

TARGET="${INSTALL_DIR}/agh"

log "AGH installer"
log "  release: ${RELEASE_REPO} ${VERSION}"
log "  platform: ${OS}/${ARCH}"
log "  archive: ${ARCHIVE_URL}"
log "  target: ${TARGET}"

if [ "$DRY_RUN" = "true" ]; then
  log "  bootstrap: $([ "$SKIP_BOOTSTRAP" = "true" ] && printf 'skipped' || printf 'interactive when /dev/tty is available')"
  log "dry run complete"
  exit 0
fi

command -v curl >/dev/null 2>&1 || fail "curl is required"
command -v tar >/dev/null 2>&1 || fail "tar is required"

if [ "$VERSION" = "latest" ]; then
  VERSION="$(resolve_latest_release_tag)"
  BASE_URL="https://github.com/${RELEASE_REPO}/releases/download/${VERSION}"
  ARCHIVE_URL="${BASE_URL}/${ARCHIVE_NAME}"
  CHECKSUM_URL="${BASE_URL}/checksums.txt"
  BUNDLE_URL="${BASE_URL}/checksums.txt.sigstore.json"
  log "resolved latest release to ${VERSION}"
fi

if command -v sha256sum >/dev/null 2>&1; then
  CHECKSUM_CMD="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
  CHECKSUM_CMD="shasum"
else
  fail "sha256sum or shasum is required to verify the download"
fi

TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/agh-install.XXXXXX")"
TMP_TARGET=""

cleanup() {
  rm -rf "$TMP_DIR"
  if [ "$TMP_TARGET" != "" ] && [ -f "$TMP_TARGET" ]; then
    rm -f "$TMP_TARGET"
  fi
}

trap cleanup EXIT INT TERM

resolve_cosign

ARCHIVE_PATH="${TMP_DIR}/${ARCHIVE_NAME}"
CHECKSUM_PATH="${TMP_DIR}/checksums.txt"
BUNDLE_PATH="${TMP_DIR}/checksums.txt.sigstore.json"
EXTRACT_DIR="${TMP_DIR}/extract"

log "downloading archive"
curl -fsSL "$ARCHIVE_URL" -o "$ARCHIVE_PATH"
curl -fsSL "$CHECKSUM_URL" -o "$CHECKSUM_PATH"
curl -fsSL "$BUNDLE_URL" -o "$BUNDLE_PATH"

log "verifying checksum provenance"
"$COSIGN_BIN" verify-blob "$CHECKSUM_PATH" \
  --bundle "$BUNDLE_PATH" \
  --certificate-identity-regexp "$COSIGN_CERT_IDENTITY_REGEXP" \
  --certificate-oidc-issuer "$COSIGN_CERT_OIDC_ISSUER" >/dev/null

CHECKSUM_LINE="$(awk -v file="$ARCHIVE_NAME" '$2 == file { print; found=1; exit } END { if (!found) exit 1 }' "$CHECKSUM_PATH" || true)"
[ "$CHECKSUM_LINE" != "" ] || fail "checksums.txt does not include ${ARCHIVE_NAME}"

log "verifying checksum"
if [ "$CHECKSUM_CMD" = "sha256sum" ]; then
  printf '%s\n' "$CHECKSUM_LINE" | (cd "$TMP_DIR" && sha256sum -c - >/dev/null)
else
  printf '%s\n' "$CHECKSUM_LINE" | (cd "$TMP_DIR" && shasum -a 256 -c - >/dev/null)
fi

mkdir -p "$EXTRACT_DIR"
tar -xzf "$ARCHIVE_PATH" -C "$EXTRACT_DIR"

BIN_PATH="$(find "$EXTRACT_DIR" -type f -name agh | head -n 1)"
[ "$BIN_PATH" != "" ] || fail "archive did not contain an agh binary"

mkdir -p "$INSTALL_DIR"
TMP_TARGET="${INSTALL_DIR}/.agh.tmp.$$"
cp "$BIN_PATH" "$TMP_TARGET"
chmod 0755 "$TMP_TARGET"
mv "$TMP_TARGET" "$TARGET"
TMP_TARGET=""

log "installed ${TARGET}"
"$TARGET" version >/dev/null
log "verified agh version"

case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    log "warning: ${INSTALL_DIR} is not on PATH"
    log "add it to PATH or run ${TARGET} directly"
    ;;
esac

if [ "$SKIP_BOOTSTRAP" = "true" ]; then
  log "bootstrap skipped"
  log "next: agh install"
  exit 0
fi

if (: </dev/tty >/dev/tty) 2>/dev/null; then
  log "starting agh install"
  "$TARGET" install </dev/tty >/dev/tty
else
  log "no interactive terminal detected; run this next:"
  log "  agh install"
fi
