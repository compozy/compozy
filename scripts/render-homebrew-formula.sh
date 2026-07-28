#!/usr/bin/env bash
set -euo pipefail

fail() {
  printf 'homebrew formula renderer: %s\n' "$*" >&2
  exit 1
}

release_repo=""
release_version=""
release_tag=""
checksums_path=""
output_path=""

while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --release-repo) release_repo="${2:-}"; shift 2 ;;
    --release-version) release_version="${2:-}"; shift 2 ;;
    --release-tag) release_tag="${2:-}"; shift 2 ;;
    --checksums) checksums_path="${2:-}"; shift 2 ;;
    --output) output_path="${2:-}"; shift 2 ;;
    *) fail "unknown argument: $1" ;;
  esac
done

[[ "$release_repo" =~ ^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$ ]] || fail "invalid release repository"
[[ "$release_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "stable formula version must be unprefixed SemVer"
[[ "$release_tag" == "v${release_version}" ]] || fail "release tag and version do not match"
[[ -f "$checksums_path" ]] || fail "checksums file does not exist"
[[ -n "$output_path" ]] || fail "output path is required"

archive_record() {
  local platform="$1"
  local arch="$2"
  local pattern="^compozy_.*${platform}_${arch}\\.tar\\.gz$"
  local records
  records="$(awk -v pattern="$pattern" '$2 ~ pattern { print $1 " " $2 }' "$checksums_path")"
  [[ "$(wc -l <<<"$records" | tr -d ' ')" == "1" ]] || fail "expected one ${platform}/${arch} archive"
  local checksum filename
  read -r checksum filename <<<"$records"
  [[ "$checksum" =~ ^[0-9a-fA-F]{64}$ ]] || fail "invalid checksum for ${filename}"
  [[ "$filename" =~ ^[A-Za-z0-9._-]+$ ]] || fail "unsafe archive filename"
  printf '%s %s\n' "$checksum" "$filename"
}

read -r darwin_amd64_sha darwin_amd64_file < <(archive_record darwin x86_64)
read -r darwin_arm64_sha darwin_arm64_file < <(archive_record darwin arm64)
read -r linux_amd64_sha linux_amd64_file < <(archive_record linux x86_64)
read -r linux_arm64_sha linux_arm64_file < <(archive_record linux arm64)

mkdir -p "$(dirname "$output_path")"
cat >"$output_path" <<FORMULA
class Compozy < Formula
  desc "CompozyOS agent operating system"
  homepage "https://compozy.com"
  version "${release_version}"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/${release_repo}/releases/download/${release_tag}/${darwin_arm64_file}"
      sha256 "${darwin_arm64_sha}"
    else
      url "https://github.com/${release_repo}/releases/download/${release_tag}/${darwin_amd64_file}"
      sha256 "${darwin_amd64_sha}"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/${release_repo}/releases/download/${release_tag}/${linux_arm64_file}"
      sha256 "${linux_arm64_sha}"
    else
      url "https://github.com/${release_repo}/releases/download/${release_tag}/${linux_amd64_file}"
      sha256 "${linux_amd64_sha}"
    end
  end

  def install
    bin.install "compozy"
  end

  test do
    system "#{bin}/compozy", "version"
  end
end
FORMULA
