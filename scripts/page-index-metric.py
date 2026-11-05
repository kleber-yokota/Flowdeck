#!/usr/bin/env python3
"""
page-index-metric.py — the page index load lines (machine-readable).

The metric is the routing precision of the page index (index.md). A folder is
a page; a folder is correctly separated when a task routes to exactly the page
it needs. Scope: code folders under `core/` and `web/` (a folder with a .go file) —
the index covers only those trees (index.md's scope line); the entry groups
(`main` · `workflows` · `cmd/`) and the index's non-code folders
(diagrams · tasks · ui) are out of scope by construction.

Run:
  python3 scripts/page-index-metric.py

GATE (exits non-zero when crossed):
  routing    a backticked `covers` key appearing in more than one page —
             the index cannot decide which page to load

SIGNAL (printed for the agent; never fails):
  breadth    `covers` entries per page — a page naming more than one set of
             intentions is a bucket                      (flag > BREADTH_MAX)
  load       README + top-level .go lines per code folder — the folder twin
             of the 300-line file rule                   (flag > LOAD_MAX)
  unindexed  code folders with a .go file and no page in index.md

The thresholds below are the baseline. Changing one means changing the value
here and the load-path line in code.md together (the structure-gate.sh rule).
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

# --- thresholds (the baseline; change here and in code.md together) ---
BREADTH_MAX = 8   # `covers` entries per page before a page names more than one set
LOAD_MAX = 1000   # README + top-level .go lines before a page is heavy to load

ROOT = Path(__file__).resolve().parent.parent
INDEX = ROOT / "index.md"
IGNORED = {".git", "node_modules"}
SCOPE_DIRS = ("core", "web")  # the index covers core/ and web/ (index.md's scope line)


def parse_pages(index: Path) -> dict[str, str]:
    """Return {page_path: covers_string} from the index '## pages' table."""
    pages: dict[str, str] = {}
    in_pages = False
    for line in index.read_text().splitlines():
        s = line.strip()
        if s.startswith("## "):
            in_pages = (s == "## pages")
            continue
        if in_pages and s.startswith("|") and "`" in s:
            cells = [c.strip() for c in s.strip("|").split("|")]
            if len(cells) >= 2:
                m = re.search(r"`([^`]+)`", cells[0])
                if m:
                    pages[m.group(1)] = cells[1]
    return pages


def is_code_folder(folder: Path) -> bool:
    """A code folder is a folder that holds a .go file."""
    return folder.is_dir() and any(folder.glob("*.go"))


def routing(pages: dict[str, str]) -> tuple[dict[str, set[str]], int]:
    """Backticked `covers` keys: {token: {pages}}. Return (ambiguous, total)."""
    keys: dict[str, set[str]] = {}
    for page, covers in pages.items():
        for token in re.findall(r"`([^`]+)`", covers):
            keys.setdefault(token, set()).add(page)
    ambiguous = {t: ps for t, ps in keys.items() if len(ps) > 1}
    return ambiguous, len(keys)


def breadth(pages: dict[str, str]) -> list[tuple[int, str]]:
    """Covers entries per page, sorted widest first."""
    rows = [(covers.count("·") + 1, page) for page, covers in pages.items()]
    rows.sort(reverse=True)
    return rows


def load_cost(page: Path) -> tuple[int, int, int]:
    """Return (total, readme_lines, go_lines) for a page's folder (top-level .go only)."""
    readme = len(page.read_text().splitlines()) if page.exists() else 0
    go = sum(len(f.read_text().splitlines()) for f in page.parent.glob("*.go"))
    return readme + go, readme, go


def in_scope(page: str) -> bool:
    """A page is in scope when it names a folder under one of SCOPE_DIRS."""
    return page.split("/")[0] in SCOPE_DIRS


def unindexed(root: Path, pages: dict[str, str]) -> list[str]:
    """Code folders (a .go file) under a scope dir that have no page."""
    have = {p.replace("README.md", "").rstrip("/") for p in pages}
    code_dirs: set[str] = set()
    for scope in SCOPE_DIRS:
        base = root / scope
        if not base.is_dir():
            continue
        for f in base.rglob("*.go"):
            if any(part in IGNORED for part in f.parts):
                continue
            code_dirs.add(str(f.parent.relative_to(root)))
    return sorted(d for d in code_dirs if d not in have)


def main() -> int:
    pages = parse_pages(INDEX)
    code = {p: c for p, c in pages.items()
            if in_scope(p) and is_code_folder((ROOT / p).parent)}
    scope_label = " · ".join(d + "/" for d in SCOPE_DIRS)

    print(f"page-index-metric — the page index load lines")
    print(f"scope: {len(code)} code pages under {scope_label} ({INDEX.name})\n")

    # --- GATE: routing ---
    ambiguous, total_keys = routing(code)
    print("ROUTING (gate)")
    if ambiguous:
        for token, ps in sorted(ambiguous.items()):
            print(f"  {token!r} is covered by: {sorted(ps)}")
        print(f"  FAIL — {len(ambiguous)} key(s) route to more than one page "
              f"({total_keys} distinct keys)")
    else:
        print(f"  ok — no backticked `covers` key appears in more than one "
              f"page ({total_keys} distinct keys)\n")

    # --- SIGNAL: breadth ---
    print(f"BREADTH (signal · `covers` entries per page · flag > {BREADTH_MAX})")
    for n, page in breadth(code):
        flag = "  [over]" if n > BREADTH_MAX else ""
        print(f"  {n:2d}  {page}{flag}")
    print()

    # --- SIGNAL: load cost ---
    print(f"LOAD (signal · README + top-level .go lines · flag > {LOAD_MAX})")
    rows = []
    for page in code:
        total, readme, go = load_cost(ROOT / page)
        rows.append((total, readme, go, page))
    for total, readme, go, page in sorted(rows, reverse=True):
        flag = "  [over]" if total > LOAD_MAX else ""
        print(f"  {total:5d}  (readme {readme:3d} + go {go:5d})  {page}{flag}")
    print()

    # --- SIGNAL: unindexed code folders ---
    missing = unindexed(ROOT, pages)
    print(f"UNINDEXED (signal · code folders with a .go file, no page · {scope_label} only)")
    if missing:
        for d in missing:
            print(f"  {d or '.'}")
    else:
        print("  (none)")

    return 1 if ambiguous else 0


if __name__ == "__main__":
    sys.exit(main())
