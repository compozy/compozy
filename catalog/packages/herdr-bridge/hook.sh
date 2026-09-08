#!/bin/sh
# Entrada de todo hook. Precisa devolver em milissegundos: o daemon do
# CompozyOS despacha os hooks de uma extensao em serie e descarta a fila
# quando o run termina — e um loop de custo zero dispara seus 5 eventos em
# ~200 ms, menos do que o Python leva para subir. Entao aqui so se grava o
# payload num spool (rename atomico) e se dispara o drenador em background.
set -u
dir="${XDG_STATE_HOME:-$HOME/.local/state}/herdr-bridge/spool"
mkdir -p "$dir" || exit 0
f="$dir/$$-$(od -An -N4 -tu4 /dev/urandom | tr -d ' ').json"
cat > "$f.tmp" && mv "$f.tmp" "$f"
here="$(cd "$(dirname "$0")" && pwd)"
nohup "$here/bridge.py" --drain >/dev/null 2>&1 &
exit 0
