#!/usr/bin/env bash
#
# test.sh — the booted test suite with a fresh database (the CI race job, local).
#
# The suite is SQLite on disk (the .env.test database). The file survives the
# test process, so a fresh run deletes it first: the assertions tolerate runs
# left by earlier tests (they filter by execution id), but a clean baseline
# means no residual runs to quiesce. CI does the same (rm before the race job);
# this is the local equivalent of that one step.
#
# Usage:
#   bash scripts/test.sh                            # the full booted suite
#   bash scripts/test.sh -run TestLive ./core/reads/ # pass through to go test
#
# The plain unit layer (no env — the booted tests skip) is just:
#   go test -count=1 ./...
#
set -euo pipefail
cd "$(dirname "$0")/.."

set -a && source .env.test && set +a

# Fresh database: drop the on-disk SQLite file (and its -wal / -shm sidecars)
# so the run starts from a clean system database.
case "${DBOS_SYSTEM_DATABASE_URL}" in
  sqlite:*)
    dbfile="${DBOS_SYSTEM_DATABASE_URL#sqlite:}"
    rm -f "${dbfile}" "${dbfile}-wal" "${dbfile}-shm"
    ;;
esac

if [ "$#" -gt 0 ]; then
  exec go test -race -count=2 -shuffle=on -p 1 "$@"
fi
exec go test -race -count=2 -shuffle=on -p 1 ./...
