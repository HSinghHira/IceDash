#!/bin/sh
# Installs IceDash on a CasaOS box, as the main UI at this server's address.
# The stock CasaOS UI stays at /casaos/. Run it again to repair or upgrade.
#
#   wget -qO- https://github.com/HSinghHira/IceDash/releases/latest/download/install-casaos.sh | sudo sh
#
# To remove it: sudo /opt/icedash/current/uninstall.sh
set -eu

REPO=HSinghHira/IceDash
BASE="${ICEDASH_RELEASE_URL:-https://github.com/$REPO/releases/latest/download}"
HOME_DIR=/opt/icedash
UNIT=/etc/systemd/system/icedash.service

say() { printf '%s\n' "$*"; }
fail() { printf 'IceDash install: %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" = 0 ] || fail "run this with sudo."
command -v systemctl >/dev/null 2>&1 || fail "this needs systemd."
[ -f /var/run/casaos/management.url ] || fail "CasaOS isn't running here (no /var/run/casaos/management.url). Install CasaOS first: https://casaos.io"
if grep -q '^ID=zimaos' /etc/os-release 2>/dev/null; then
  fail "this is ZimaOS: install the IceDash module instead (see the README)."
fi

case "$(uname -m)" in
  x86_64 | amd64) ARCH=amd64 ;;
  aarch64 | arm64) ARCH=arm64 ;;
  *) fail "unsupported processor $(uname -m); IceDash runs on amd64 and arm64." ;;
esac

fetch() {
  if command -v wget >/dev/null 2>&1; then
    wget -qO "$2" "$1"
  elif command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "$2" "$1"
  else
    fail "needs wget or curl."
  fi
}

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
NAME="icedash-casaos-$ARCH.tar.gz"
say "Downloading $NAME..."
fetch "$BASE/$NAME" "$TMP/$NAME" || fail "download failed: $BASE/$NAME"
fetch "$BASE/$NAME.sha256" "$TMP/$NAME.sha256" || fail "download failed: $BASE/$NAME.sha256"
WANT=$(cut -d' ' -f1 "$TMP/$NAME.sha256")
GOT=$(sha256sum "$TMP/$NAME" | cut -d' ' -f1)
[ "$WANT" = "$GOT" ] || fail "the download doesn't match its checksum."

mkdir -p "$TMP/x"
tar -xzmf "$TMP/$NAME" -C "$TMP/x" --no-same-owner
chmod 0755 "$TMP/x/icedash" "$TMP/x/uninstall.sh" 2>/dev/null || true
[ -x "$TMP/x/icedash" ] && [ -f "$TMP/x/VERSION" ] || fail "the release is incomplete."
VERSION=$(cat "$TMP/x/VERSION")
case "$VERSION" in *[!0-9A-Za-z.+-]* | "") fail "odd version '$VERSION' in the release." ;; esac

say "Installing IceDash v$VERSION..."
mkdir -p "$HOME_DIR/releases"
chmod 755 "$HOME_DIR" "$HOME_DIR/releases"
rm -rf "$HOME_DIR/releases/$VERSION"
mv "$TMP/x" "$HOME_DIR/releases/$VERSION"
chown -R root:root "$HOME_DIR/releases/$VERSION"
ln -sfn "$HOME_DIR/releases/$VERSION" "$HOME_DIR/current.next"
mv -T "$HOME_DIR/current.next" "$HOME_DIR/current"
# Keep only this release (a later self-update keeps the previous one too).
for d in "$HOME_DIR"/releases/*; do
  [ "$d" = "$HOME_DIR/releases/$VERSION" ] || rm -rf "$d"
done

install -m 0644 "$HOME_DIR/current/icedash.service" "$UNIT"
systemctl daemon-reload
systemctl enable icedash.service >/dev/null 2>&1
systemctl restart icedash.service

# Wait until it has taken over the main address.
i=0
while [ "$i" -lt 30 ]; do
  if grep -q '"/":"http://127.0.0.1' /var/run/casaos/routes.json 2>/dev/null &&
    [ "$(sed -n 's/.*"\/":"\([^"]*\)".*/\1/p' /var/run/casaos/routes.json)" != "$(cat /var/run/casaos/static.url 2>/dev/null)" ]; then
    break
  fi
  sleep 1
  i=$((i + 1))
done

IP=$(hostname -I 2>/dev/null | cut -d' ' -f1)
say ""
say "IceDash v$VERSION is running."
say "  Open:            http://${IP:-this-server}/"
say "  CasaOS's own UI: http://${IP:-this-server}/casaos/"
say "  Remove it:       sudo $HOME_DIR/current/uninstall.sh"
if [ "$i" -ge 30 ]; then
  say ""
  say "It hasn't taken over the main address yet. Check: sudo journalctl -u icedash -n 50"
fi
