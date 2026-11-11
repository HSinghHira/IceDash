#!/bin/sh
# Checks out lintux-modkit at the commit named in lintux-modkit.ref ("<tag> <commit>").
#   sh scripts/modkit.sh [<dir>]      default .modkit
# Refuses a checkout with local changes, so every build uses exactly the pinned code.
set -eu
cd "$(dirname "$0")/.."
DIR="${1:-.modkit}"
read -r TAG COMMIT < lintux-modkit.ref
if [ ! -d "$DIR/.git" ]; then
  # Its shell scripts run on Linux: keep LF line endings, also on a Windows checkout.
  git -c core.autocrlf=false clone -q https://github.com/chicohaager/lintux-modkit.git "$DIR"
  git -C "$DIR" config core.autocrlf false
fi
if [ "$(git -C "$DIR" rev-parse HEAD 2>/dev/null || true)" != "$COMMIT" ]; then
  git -C "$DIR" fetch -q --tags origin
  git -C "$DIR" -c advice.detachedHead=false checkout -q "$COMMIT"
fi
if [ -n "$(git -C "$DIR" status --porcelain)" ]; then
  echo "refused: $DIR has local changes; it must be lintux-modkit $TAG ($COMMIT)" >&2
  exit 1
fi
echo "lintux-modkit $TAG ($COMMIT)"
