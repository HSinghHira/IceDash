#!/bin/sh
# Removes IceDash from ZimaOS or CasaOS and makes sure this server's address
# opens the stock dashboard again, whichever IceDash version was installed.
# Settings and notifications in /DATA/.icedash stay unless you add --purge.
# Apps you installed stay either way.
#
#   curl -fsSL https://github.com/HSinghHira/IceDash/releases/latest/download/uninstall.sh | sudo sh
#   curl -fsSL https://github.com/HSinghHira/IceDash/releases/latest/download/uninstall.sh | sudo sh -s -- --purge
set -eu

# All in one function, so `curl ... | sh` reads the whole script before running it.
main() {

say() { printf '%s\n' "$*"; }
fail() { printf 'IceDash uninstall: %s\n' "$*" >&2; exit 1; }

purge=0
for a in "$@"; do
  case "$a" in
    --purge) purge=1 ;;
    *) fail "usage: uninstall.sh [--purge]" ;;
  esac
done
[ "$(id -u)" = 0 ] || fail "run this with sudo."

if grep -q '^ID=zimaos' /etc/os-release 2>/dev/null || command -v zpkg >/dev/null 2>&1; then
  say "Stopping IceDash..."
  # Stop it first: while it runs it keeps its "/" route in the gateway.
  systemctl stop icedash 2>/dev/null || true
  if [ -x /usr/bin/icedash ]; then
    # Removes the boot watchdog units, which zpkg doesn't know about.
    /usr/bin/icedash --uninstall >/dev/null 2>&1 || true
  fi
  # Give "/" back to ZimaOS here as well, in case the version installed
  # couldn't (none there is fine).
  if [ -S /run/caddy-admin.sock ]; then
    curl -fsS --unix-socket /run/caddy-admin.sock -X DELETE http://localhost/id/icedash-home >/dev/null 2>&1 || true
  fi
  say "Removing the module..."
  zpkg remove icedash </dev/null || say "zpkg couldn't remove icedash (was it installed?)"

  # Check "/" through the gateway; a gateway restart drops any route left.
  PORT=80
  if [ -f /var/run/casaos/management.url ]; then
    P=$(curl -fsS "$(cat /var/run/casaos/management.url)/v1/gateway/port" 2>/dev/null | sed -n 's/.*"data":"\([0-9]*\)".*/\1/p')
    if [ -n "$P" ]; then PORT=$P; fi
  fi
  if curl -sSI "http://127.0.0.1:$PORT/" 2>/dev/null | grep -qi '^x-icedash-front:'; then
    say "Restarting the ZimaOS gateway to give its address back..."
    systemctl restart zimaos-gateway || true
  fi

  if [ "$purge" = 1 ]; then
    rm -rf /DATA/.icedash
    say "Removed IceDash and its settings."
  else
    say "Removed IceDash. Its settings stay in /DATA/.icedash (add --purge to remove them)."
  fi
  say "The ZimaOS dashboard is back at this server's address."
  exit 0
fi

if [ -f /var/run/casaos/management.url ] || [ -d /opt/icedash ]; then
  if [ -x /opt/icedash/current/uninstall.sh ]; then
    if [ "$purge" = 1 ]; then
      exec sh /opt/icedash/current/uninstall.sh --purge
    fi
    exec sh /opt/icedash/current/uninstall.sh
  fi
  fail "IceDash isn't installed here (no /opt/icedash/current/uninstall.sh)."
fi

fail "this isn't ZimaOS or CasaOS."
}

main "$@"
