#!/usr/bin/env bash
set -euo pipefail
# Vendor the control-plane hooks from the canonical skills repo
# (single source of truth: github.com/vanducng/skills) into ./hooks.
#
# This committed copy keeps vd-cli self-contained for `vd install hooks` and its
# tests. hooks/.vendored-from records the exact skills commit it came from; the
# hooks-drift CI job re-runs this script and fails if ./hooks no longer matches
# that commit — so the vendored copy can never silently diverge from upstream.
#
#   scripts/sync-hooks.sh --ref main   # bump to skills@main, record the new pin
#   scripts/sync-hooks.sh              # re-materialize the recorded pin (verify / CI)
#   scripts/sync-hooks.sh --src PATH   # vendor from a local skills checkout

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEST="$REPO_ROOT/hooks"
PIN_FILE="$DEST/.vendored-from"
SKILLS_URL="${SKILLS_URL:-https://github.com/vanducng/skills.git}"

CLEANUP="" STAGE=""
cleanup() { [ -n "$CLEANUP" ] && rm -rf "$CLEANUP"; [ -n "$STAGE" ] && rm -rf "$STAGE"; return 0; }
trap cleanup EXIT

REF="" SRC=""
while [ $# -gt 0 ]; do
  case "$1" in
    --ref) [ $# -ge 2 ] || { echo "--ref requires a value" >&2; exit 2; }; REF="$2"; shift 2 ;;
    --src) [ $# -ge 2 ] || { echo "--src requires a value" >&2; exit 2; }; SRC="$2"; shift 2 ;;
    -h|--help) grep '^#' "$0" | sed 's/^#\{1,\} \{0,1\}//'; exit 0 ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done

# No explicit ref → re-materialize the recorded pin (verify mode); fall back to main.
if [ -z "$REF" ]; then
  if [ -f "$PIN_FILE" ]; then REF="$(tr -d '[:space:]' < "$PIN_FILE")"; else REF="main"; fi
fi

if [ -z "$SRC" ]; then
  CLEANUP="$(mktemp -d)"
  # Blobless + no-checkout: fetch metadata only, then materialize just the pinned
  # commit. Works for arbitrary SHAs (unlike --depth 1) while skipping full history.
  git clone --quiet --filter=blob:none --no-checkout "$SKILLS_URL" "$CLEANUP"
  git -C "$CLEANUP" checkout --quiet "$REF"
  SRC="$CLEANUP"
fi

[ -d "$SRC/hooks" ] || { echo "no hooks/ under skills source: $SRC" >&2; exit 1; }

# Vendor the COMMITTED HEAD (git archive), never the working tree — so a dirty
# local --src checkout can't leak uncommitted edits into the vendored copy.
RESOLVED="$(git -C "$SRC" rev-parse HEAD)"
STAGE="$(mktemp -d)"
git -C "$SRC" archive HEAD hooks | tar -x -C "$STAGE"
rsync -a --delete --exclude='__pycache__' --exclude='.vendored-from' "$STAGE/hooks/" "$DEST/"
printf '%s\n' "$RESOLVED" > "$PIN_FILE"

# temp dirs ($CLEANUP, $STAGE) are removed by the EXIT trap
echo "vendored hooks from skills@${RESOLVED}"
