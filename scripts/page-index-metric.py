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
  python3 scripts/page-index-metric.py              # the readings
  python3 scripts/page-index-metric.py --self-test  # the fixture checks

GATE (exits non-zero when crossed):
  routing    a backticked `covers` key appearing in more than one page —
             the index cannot decide which page to load

SIGNAL (printed for the agent; never fails):
  index      the pages table itself — dead rows (no code folder behind),
             out-of-scope rows (outside core/ · web/), duplicate rows
  breadth    top-level `covers` entries per page (a middle dot inside
             parens is a sub-list) — a page naming more than one set of
             intentions is a bucket                      (flag > BREADTH_MAX)
  load       README + top-level production .go lines per code folder
             (no _test.go — the test load is the suite's) (flag > LOAD_MAX)
  file       any .go file under core/ · web/ (tests included) over the
             300-line file rule — a file this long has outgrown one
             responsibility (code.md debt)
  unindexed  code folders with a .go file and no page in index.md

The thresholds below are the baseline. Changing one means changing the value
here and the load-path line in code.md together (the structure-gate.sh rule).
"""

from __future__ import annotations

import re
import sys
import tempfile
from pathlib import Path

# --- thresholds (the baseline; change here and in code.md together) ---
BREADTH_MAX = 8   # `covers` entries per page before a page names more than one set
LOAD_MAX = 1000   # README + top-level production .go lines (no _test.go) before
                  # a page is heavy to load
LINE_MAX = 300    # Go file lines before it has outgrown one responsibility (code.md)

ROOT = Path(__file__).resolve().parent.parent
INDEX = ROOT / "index.md"
IGNORED = {".git", "node_modules"}
SCOPE_DIRS = ("core", "web")  # the index covers core/ and web/ (index.md's scope line)


def parse_pages(index: Path) -> tuple[dict[str, str], list[str]]:
    """Return ({page_path: covers_string}, duplicate_rows) from '## pages'."""
    pages: dict[str, str] = {}
    duplicates: list[str] = []
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
                    page = m.group(1)
                    if page in pages and page not in duplicates:
                        duplicates.append(page)
                    pages[page] = cells[1]
    return pages, duplicates


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


def covers_entries(covers: str) -> int:
    """Top-level `covers` entries (a middle dot inside parens is a sub-list)."""
    if not covers.strip():
        return 0
    depth = 0
    entries = 1
    for ch in covers:
        if ch == "(":
            depth += 1
        elif ch == ")":
            depth -= 1
        elif ch == "·" and depth == 0:
            entries += 1
    return entries


def breadth(pages: dict[str, str]) -> list[tuple[int, str]]:
    """Top-level covers entries per page, sorted widest first."""
    rows = [(covers_entries(covers), page) for page, covers in pages.items()]
    rows.sort(reverse=True)
    return rows


def load_cost(page: Path) -> tuple[int, int, int]:
    """Return (total, readme_lines, prod_go_lines) for a page's folder.

    Top-level production .go only (no _test.go — the test load is the
    suite's, not the page's)."""
    readme = len(page.read_text().splitlines()) if page.exists() else 0
    go = sum(len(f.read_text().splitlines()) for f in page.parent.glob("*.go")
             if not f.name.endswith("_test.go"))
    return readme + go, readme, go


def scope_go_files(root: Path) -> list[Path]:
    """Every .go file under the scope dirs (ignored dirs skipped)."""
    files: list[Path] = []
    for scope in SCOPE_DIRS:
        base = root / scope
        if not base.is_dir():
            continue
        for f in base.rglob("*.go"):
            if not any(part in IGNORED for part in f.parts):
                files.append(f)
    return files


def file_lines(root: Path) -> list[tuple[int, str]]:
    """(lines, relpath) for every .go file under the scope dirs (tests included)."""
    rows = [(len(f.read_text().splitlines()), str(f.relative_to(root)))
            for f in scope_go_files(root)]
    rows.sort(key=lambda row: (-row[0], row[1]))
    return rows


def in_scope(page: str) -> bool:
    """A page is in scope when it names a folder under one of SCOPE_DIRS."""
    return page.split("/")[0] in SCOPE_DIRS


def unindexed(root: Path, pages: dict[str, str]) -> list[str]:
    """Code folders (a .go file) under a scope dir that have no page row."""
    have = {str(Path(p).parent) for p in pages}  # the folder the row claims
    dirs = {str(f.parent.relative_to(root)) for f in scope_go_files(root)}
    return sorted(d for d in dirs if d not in have)


def dead_pages(pages: dict[str, str], root: Path) -> list[str]:
    """In-scope rows whose folder has no .go file (a dead page)."""
    return sorted(p for p in pages
                  if in_scope(p) and not is_code_folder((root / p).parent))


def out_of_scope_pages(pages: dict[str, str]) -> list[str]:
    """Rows naming a folder outside the scope dirs (the index covers core/ · web/)."""
    return sorted(p for p in pages if not in_scope(p))


def render(items: list[str]) -> str:
    """A signal block's item list ('(none)' when empty)."""
    return "    " + "\n    ".join(items) if items else "    (none)"


def self_test() -> int:
    """Fixture checks for the pure functions (no repo, no network)."""
    failures = 0

    def check(name: str, cond: bool) -> None:
        nonlocal failures
        print(f"  {name}  {'ok' if cond else 'FAIL'}")
        if not cond:
            failures += 1

    # parse_pages: rows, duplicates (last wins), section bound
    with tempfile.TemporaryDirectory() as td:
        idx = Path(td) / "index.md"
        idx.write_text(
            "# index\n\n## pages\n\n"
            "| page | covers |\n|---|---|\n"
            "| `core/a/README.md` | x · `shared` |\n"
            "| `core/a/README.md` | x · `other` |\n"
            "| `cmd/tool/README.md` | y |\n"
            "| `core/ghost/README.md` | z |\n"
            "\n## scope\n\nnothing\n"
        )
        pages, dups = parse_pages(idx)
        check("parse_pages rows", set(pages) == {
            "core/a/README.md", "cmd/tool/README.md", "core/ghost/README.md"})
        check("parse_pages duplicate + last wins",
              dups == ["core/a/README.md"] and pages["core/a/README.md"] == "x · `other`")

    # routing
    amb, _ = routing({"a/README.md": "`k`", "b/README.md": "`k` · x"})
    check("routing ambiguous", amb == {"k": {"a/README.md", "b/README.md"}})
    amb2, _ = routing({"a/README.md": "`k` · `k`"})
    check("routing repeat-in-one-page ok", amb2 == {})

    # in_scope
    check("in_scope", in_scope("core/a/README.md") and in_scope("web/b/README.md")
          and not in_scope("cmd/a/README.md") and not in_scope("workflows/a/README.md"))

    # breadth: top-level entries (parens are sub-lists), empty cell is 0
    check("breadth count", breadth({"p": "a · b · c"})[0][0] == 3)
    check("breadth paren sub-list", breadth({"p": "a · b (x · y) · c"})[0][0] == 3)
    check("breadth empty cell", breadth({"p": ""})[0][0] == 0)

    # load_cost: README + top-level production .go (no _test.go, no subfolders)
    with tempfile.TemporaryDirectory() as td:
        folder = Path(td) / "core" / "a"
        (folder / "sub").mkdir(parents=True)
        (folder / "README.md").write_text("l1\nl2\n")
        (folder / "a.go").write_text("package a\n")
        (folder / "b.go").write_text("package a\n")
        (folder / "b_test.go").write_text("package a\n")
        (folder / "sub" / "c.go").write_text("package sub\n")
        check("load_cost prod top-level only (tests excluded)",
              load_cost(folder / "README.md") == (4, 2, 2))

    # dead / out-of-scope / unindexed
    with tempfile.TemporaryDirectory() as td:
        root = Path(td)
        (root / "core" / "a").mkdir(parents=True)
        (root / "core" / "a" / "a.go").write_text("package a\n")
        (root / "core" / "b" / "sub").mkdir(parents=True)
        (root / "core" / "b" / "sub" / "s.go").write_text("package s\n")
        (root / "web" / "c").mkdir(parents=True)
        (root / "web" / "c" / "c.go").write_text("package c\n")
        (root / "workflows" / "w").mkdir(parents=True)
        (root / "workflows" / "w" / "w.go").write_text("package w\n")
        check("dead pages", dead_pages(
            {"core/a/README.md": "x", "core/ghost/README.md": "y"}, root)
            == ["core/ghost/README.md"])
        check("out-of-scope rows", out_of_scope_pages({
            "cmd/tool/README.md": "y"}) == ["cmd/tool/README.md"])
        check("unindexed scope-only",
              unindexed(root, {"core/a/README.md": "x"}) == ["core/b/sub", "web/c"])
        check("unindexed row naming a file still claims the folder",
              unindexed(root, {"core/b/sub/s.go": "y"}) == ["core/a", "web/c"])

    # file_lines: flags > LINE_MAX, tests included, scope dirs only
    with tempfile.TemporaryDirectory() as td:
        root = Path(td)
        (root / "core" / "a").mkdir(parents=True)
        (root / "core" / "a" / "long.go").write_text("x\n" * (LINE_MAX + 1))
        (root / "core" / "a" / "exact.go").write_text("x\n" * LINE_MAX)
        (root / "core" / "a" / "t_test.go").write_text("x\n" * (LINE_MAX + 1))
        (root / "web" / "c").mkdir(parents=True)
        (root / "web" / "c" / "c.go").write_text("package c\n")
        over = sorted(p for n, p in file_lines(root) if n > LINE_MAX)
        check("file_lines flags > LINE_MAX (tests included)",
              over == ["core/a/long.go", "core/a/t_test.go"])

    print(f"self-test: {'ok' if failures == 0 else f'{failures} FAILURE(S)'}")
    return 1 if failures else 0


def main() -> int:
    if "--self-test" in sys.argv[1:]:
        return self_test()

    pages, duplicates = parse_pages(INDEX)
    code = {p: c for p, c in pages.items()
            if in_scope(p) and is_code_folder((ROOT / p).parent)}
    scope_label = " · ".join(d + "/" for d in SCOPE_DIRS)

    print(f"page-index-metric — the page index load lines")
    print(f"scope: {len(code)} code pages under {scope_label} ({INDEX.name})\n")

    # --- GATE: routing (the whole pages table — a dead or out-of-scope row
    #      still routes, so its keys must be unique too) ---
    ambiguous, total_keys = routing(pages)
    print("ROUTING (gate)")
    if ambiguous:
        for token, ps in sorted(ambiguous.items()):
            print(f"  {token!r} is covered by: {sorted(ps)}")
        print(f"  FAIL — {len(ambiguous)} key(s) route to more than one page "
              f"({total_keys} distinct keys)")
    else:
        print(f"  ok — no backticked `covers` key appears in more than one "
              f"page ({total_keys} distinct keys)\n")

    # --- SIGNAL: index table health ---
    print("INDEX (signal · the pages table)")
    print("  dead pages (row behind no code folder)")
    print(render(dead_pages(pages, ROOT)))
    print(f"  out-of-scope rows (the index covers {scope_label})")
    print(render(out_of_scope_pages(pages)))
    print("  duplicate rows (same page twice · last wins)")
    print(render(duplicates))
    print()

    # --- SIGNAL: breadth ---
    print(f"BREADTH (signal · top-level `covers` entries per page · flag > {BREADTH_MAX})")
    for n, page in breadth(code):
        if n > BREADTH_MAX:
            print(f"  {n:2d}  {page}  [over]")
            print(f"      {code[page]}")
        else:
            print(f"  {n:2d}  {page}")
    print()

    # --- SIGNAL: load cost ---
    print(f"LOAD (signal · README + top-level production .go lines · flag > {LOAD_MAX})")
    rows = []
    for page in code:
        total, readme, go = load_cost(ROOT / page)
        rows.append((total, readme, go, page))
    for total, readme, go, page in sorted(rows, reverse=True):
        flag = "  [over]" if total > LOAD_MAX else ""
        print(f"  {total:5d}  (readme {readme:3d} + go {go:5d})  {page}{flag}")
    print()

    # --- SIGNAL: file length (the 300-line file rule) ---
    files = file_lines(ROOT)
    over = [(n, p) for n, p in files if n > LINE_MAX]
    print(f"FILE (signal · .go files under {scope_label} · flag > {LINE_MAX})")
    if over:
        for n, p in over:
            print(f"  {n:5d}  {p}  [over]")
        print(f"  ({len(over)} of {len(files)} files over the line)")
    else:
        print(f"  (none · {len(files)} files at or below the line)")
    print()

    # --- SIGNAL: unindexed code folders ---
    missing = unindexed(ROOT, pages)
    print(f"UNINDEXED (signal · code folders with a .go file, no page · {scope_label} only)")
    if missing:
        for d in missing:
            note = ("readme · no index row"
                    if (ROOT / d / "README.md").exists() else "no readme yet")
            print(f"  {d}  ({note})")
    else:
        print("  (none)")

    return 1 if ambiguous else 0


if __name__ == "__main__":
    sys.exit(main())
