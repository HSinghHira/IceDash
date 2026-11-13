#!/bin/sh
# Removes IceDash from a CasaOS box and gives the main address back to the
# stock CasaOS UI. Your IceDash settings and notifications in /DATA/.icedash
# stay unless you add --purge. Apps you installed stay in CasaOS either way.
#
#   sudo /opt/icedash/current/uninstall.sh [--purge]
set -eu

# This file is inside /opt/icedash, which is removed below: run from a copy.
if [ "${ICEDASH_UNINSTALL_COPY:-}" != 1 ]; then
  tmp=$(mktemp)
  cp "$0" "$tmp"
  ICEDASH_UNINSTALL_COPY=1 exec sh "$tmp" "$@"
fi
trap 'rm -f "$0"' EXIT

purge=0
for a in "$@"; do
  case "$a" in
    --purge) purge=1 ;;
    *) echo "usage: uninstall.sh [--purge]" >&2; exit 2 ;;
  esac
done
if [ "$(id -u)" != 0 ]; then
  echo "Run this with sudo." >&2
  exit 1
fi

HOME_DIR=/opt/icedash
UNIT=/etc/systemd/system/icedash.service

echo "Stopping IceDash..."
systemctl disable --now icedash.service 2>/dev/null || true
# The service gives / back as it stops; do it again in case it wasn't running.
if [ -x "$HOME_DIR/current/icedash" ]; then
  "$HOME_DIR/current/icedash" --uninstall || echo "Couldn't reach the CasaOS gateway; restarting it puts its UI back: sudo systemctl restart casaos-gateway"
fi

rm -f "$UNIT"
systemctl daemon-reload
rm -rf "$HOME_DIR"
if [ "$purge" = 1 ]; then
  rm -rf /DATA/.icedash
  echo "Removed IceDash and its settings."
else
  echo "Removed IceDash. Its settings stay in /DATA/.icedash (add --purge to remove them)."
fi
echo "The CasaOS dashboard is back at this server's address."
