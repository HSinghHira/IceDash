#!/bin/sh
# Installs IceDash on ZimaOS or CasaOS; it finds out which by itself.
# Run it again to repair or reinstall.
#
#   curl -fsSL https://icedash.hsinghhira.me/install.sh | sudo sh
#
# To remove it:
#   curl -fsSL https://icedash.hsinghhira.me/uninstall.sh | sudo sh
set -eu

# All in one function, so `curl ... | sh` reads the whole script before running it.
main() {

REPO=HSinghHira/IceDash
BASE="${ICEDASH_RELEASE_URL:-https://github.com/$REPO/releases/latest/download}"

say() { printf '%s\n' "$*"; }
fail() { printf 'IceDash install: %s\n' "$*" >&2; exit 1; }

fetch() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "$2" "$1"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$2" "$1"
  else
    fail "needs curl or wget."
  fi
}

[ "$(id -u)" = 0 ] || fail "run this with sudo."

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

if grep -q '^ID=zimaos' /etc/os-release 2>/dev/null || command -v zpkg >/dev/null 2>&1; then
  # ZimaOS: IceDash is a module (a system extension image) for zpkg.
  case "$(uname -m)" in
    x86_64 | amd64) ;;
    *) fail "ZimaOS on $(uname -m) isn't supported; IceDash's ZimaOS module is amd64 only." ;;
  esac
  NAME=icedash-amd64.raw
  say "Downloading $NAME..."
  fetch "$BASE/$NAME" "$TMP/$NAME" || fail "download failed: $BASE/$NAME"
  fetch "$BASE/$NAME.sha256" "$TMP/$NAME.sha256" || fail "download failed: $BASE/$NAME.sha256"
  WANT=$(cut -d' ' -f1 "$TMP/$NAME.sha256")
  GOT=$(sha256sum "$TMP/$NAME" | cut -d' ' -f1)
  [ "$WANT" = "$GOT" ] || fail "the download doesn't match its checksum."
  # zpkg takes the module name from the file name.
  mv "$TMP/$NAME" "$TMP/icedash.raw"

  say "Installing IceDash..."
  if [ -e /usr/bin/icedash ]; then
    zpkg install --force "$TMP/icedash.raw" </dev/null
  else
    zpkg install "$TMP/icedash.raw" </dev/null
  fi

  # Wait until its page answers through the gateway.
  i=0
  while [ "$i" -lt 60 ]; do
    if curl -fsS -o /dev/null http://127.0.0.1/modules/icedash/ 2>/dev/null; then
      break
    fi
    sleep 1
    i=$((i + 1))
  done
  IP=$(hostname -I 2>/dev/null | cut -d' ' -f1)
  say ""
  if [ "$i" -ge 60 ]; then
    say "IceDash is installed but its page isn't answering yet. Check: sudo journalctl -u icedash -n 50"
  else
    say "IceDash is running."
  fi
  say "  Open:      http://${IP:-this-server}/modules/icedash/"
  say "  Remove it: curl -fsSL https://icedash.hsinghhira.me/uninstall.sh | sudo sh"
  exit 0
fi

if [ -f /var/run/casaos/management.url ]; then
  # CasaOS: the CasaOS installer does the work.
  fetch "$BASE/install-casaos.sh" "$TMP/install-casaos.sh" || fail "download failed: $BASE/install-casaos.sh"
  ICEDASH_RELEASE_URL="$BASE" sh "$TMP/install-casaos.sh"
  exit $?
fi

fail "this isn't ZimaOS or CasaOS (no zpkg, no /var/run/casaos/management.url). IceDash runs only on those."
}

main "$@"
