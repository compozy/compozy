set -eu
umask 077
url=$1 version=$2 digest=$3
home="$HOME/.compozy"
versions="$home/runtime/versions"
lock="$home/runtime/.ssh-setup-lock"
mkdir -p "$versions"
elapsed=0
until mkdir "$lock" 2>/dev/null; do
    elapsed=$((elapsed + 1))
    [ "$elapsed" -lt 240 ] || exit 75
    sleep 1
done
scratch=
trap 'test -z "$scratch" || rm -rf "$scratch"; rmdir "$lock"' 0
trap 'exit 125' HUP INT TERM
scratch=$(mktemp -d "$versions/.ssh-install.XXXXXXXX")
archive="$scratch/release.tar.gz"

if command -v curl >/dev/null 2>&1; then
    curl -fsSL --max-time 240 -o "$archive" "$url" || exit 22
elif command -v wget >/dev/null 2>&1; then
    wget --timeout=240 --tries=1 -q -O "$archive" "$url" || exit 22
else
    printf '%s\n' 'curl or wget is required.' >&2
    exit 22
fi

if command -v sha256sum >/dev/null 2>&1; then
    actual=$(sha256sum "$archive")
else
    actual=$(shasum -a 256 "$archive")
fi
set -- $actual
[ "$1" = "$digest" ] || exit 65
tar -xOzf "$archive" compozy >"$scratch/compozy"
test -s "$scratch/compozy"
chmod 755 "$scratch/compozy"
destination="$versions/$version"
mkdir -p "$destination" "$home/bin"
mv -f "$scratch/compozy" "$destination/compozy"
ln -s "$destination/compozy" "$scratch/bin-link"
test ! -d "$home/bin/compozy"
mv -f "$scratch/bin-link" "$home/bin/compozy"
