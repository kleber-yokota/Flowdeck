#!/usr/bin/env bash
#
# structure-gate.sh — the structure load lines, machine-readable.
# Run locally or in CI:
#
#   bash scripts/structure-gate.sh
#
# The thresholds below are the baseline (the "complexity budget" line in
# code.md). Changing one means changing the value here and the code.md
# line together.
#
# GATES (the script exits non-zero when one is crossed):
#   cyclomatic   gocyclo         production function complexity   <= CYCLO_MAX
#   cognitive    gocognit        production function complexity   <= COGNIT_MAX
#   nesting      nestif         nested-if depth, production files <= NESTIF_MAX
#   cycles       sda             package import cycles            none
#   contract     goimportmaps    forbidden imports (.goimportmaps.yaml) none
# SIGNAL (printed for the agent; never fails):
#   coupling     goimportmaps    Ca / Ce / I per package (thresholds live in
#                                .goimportmaps.yaml)
#   density      go list + git   files × imports per package (the split and
#                                the responsibility readings)
#
# Not gateable yet:
#   plimsoll     god-struct load line (methods / exported fields per type) —
#                broken on Go 1.27 (export data v5); revisit when plimsoll
#                bumps x/tools.
#   interfacebloat (golangci-lint) — the repo has no interface yet (Port is
#                a struct); add the check the moment Port is drawn as one.
#
# Tool install (go install once; the binaries land next to go):
#   go install github.com/fzipp/gocyclo/cmd/gocyclo@latest
#   go install github.com/uudashr/gocognit/cmd/gocognit@latest
#   go install github.com/nakabonne/nestif/cmd/nestif@latest
#   go install github.com/sandstorm/dependency-analysis@latest
#   go install github.com/mickamy/goimportmaps/cmd/goimportmaps@latest

set -u
cd "$(dirname "$0")/.."

# ---- the load lines (thresholds) ------------------------------------------
CYCLO_MAX=9      # cyclomatic complexity, production functions
COGNIT_MAX=10    # cognitive complexity, production functions
NESTIF_MAX=5     # nested-if complexity (today's max is 5: workflows/branch)

for tool in gocyclo gocognit nestif dependency-analysis goimportmaps; do
  command -v "$tool" >/dev/null 2>&1 || {
    echo "structure-gate: $tool not found — go install it (see the header above)" >&2
    exit 2
  }
done

files=$(git ls-files '*.go' | grep -v '_test\.go$' | tr '\n' ' ')
fail=0

# 1 · cyclomatic complexity
echo "cyclomatic   <= $CYCLO_MAX"
if out=$(gocyclo -over "$CYCLO_MAX" -ignore '_test\.go$' . 2>&1); then
  echo "  ok"
else
  echo "$out" >&2; fail=1
fi

# 2 · cognitive complexity
echo "cognitive    <= $COGNIT_MAX"
if out=$(gocognit -over "$COGNIT_MAX" -ignore '_test\.go$' . 2>&1); then
  echo "  ok"
else
  echo "$out" >&2; fail=1
fi

# 3 · nesting — nestif exits 0 even with violations, so gate on its output
echo "nesting      <= $NESTIF_MAX"
if out=$(nestif --min $((NESTIF_MAX + 1)) $files 2>&1) && [ -z "$out" ]; then
  echo "  ok"
else
  echo "$out" >&2; fail=1
fi

# 4 · import cycles
echo "cycles       none"
if out=$(dependency-analysis validate . 2>&1); then
  echo "  ok — $out"
else
  echo "$out" >&2; fail=1
fi

# 5 · import contract (.goimportmaps.yaml)
echo "contract     .goimportmaps.yaml"
if out=$(goimportmaps check ./... 2>&1); then
  echo "  ok"
else
  echo "$out" >&2; fail=1
fi

# 6 · coupling metrics — SIGNAL: printed, never fails
echo "coupling     Ca / Ce / I per package (signal)"
goimportmaps ./... --metrics 2>&1 || true

# 7 · package density — SIGNAL: printed, never fails
#    files   = production .go files directly in the package directory
#    imports = unique production imports of the package
#    Reading: many files + few imports → the package can be split into
#    subpackages; few files + many imports → responsibility is concentrated
#    in the files (read the file before touching it).
echo "density      files × imports per package (signal)"
mod=$(go list -m | awk '{print $1}')
go list -f '{{.ImportPath}}|{{range .Imports}}{{.}} {{end}}' ./... |
  while IFS='|' read -r path imports; do
    dir=${path#"$mod"/}
    [ "$dir" = "$path" ] && dir=""
    if [ -z "$dir" ]; then
      files=$(git ls-files '*.go' | grep -v '_test\.go$' | grep -cv '/')
    else
      nf=$(echo "$dir" | awk -F'/' '{print NF + 1}')
      files=$(git ls-files "$dir" | grep -v '_test\.go$' | awk -F'/' -v n="$nf" 'NF == n' | grep -c '\.go$')
    fi
    imp=$(echo "$imports" | tr ' ' '\n' | grep -c '[^ ]')
    printf '  %-24s files=%-3d imports=%d\n' "$path" "$files" "$imp"
  done

exit $fail
