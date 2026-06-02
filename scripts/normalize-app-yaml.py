#!/usr/bin/env python3
"""Collapse excessive blank lines in config/app.yaml (preserve values and comments)."""
import re
import sys
from pathlib import Path


def is_section_starter(line: str) -> bool:
    if not line.strip():
        return False
    if line.startswith("# ====="):
        return True
    if re.match(r"^[a-z_][a-z0-9_]*:\s*$", line):
        return True
    if re.match(r"^      - name:", line):
        return True
    if re.match(r"^# [a-z_].*:\s", line) and not line.startswith("#   "):
        return True
    return False


def normalize(lines: list[str]) -> list[str]:
    out: list[str] = []
    for i, line in enumerate(lines):
        stripped = line.rstrip()
        if stripped == "":
            j = i + 1
            while j < len(lines) and lines[j].strip() == "":
                j += 1
            if j < len(lines) and is_section_starter(lines[j]):
                if out and out[-1] != "":
                    out.append("")
            continue
        out.append(stripped)
    return out


def main() -> int:
    root = Path(__file__).resolve().parents[1]
    path = Path(sys.argv[1]) if len(sys.argv) > 1 else root / "config" / "app.yaml"
    raw = path.read_text(encoding="utf-8")
    lines = raw.splitlines()
    out = normalize(lines)
    path.write_text("\n".join(out).rstrip() + "\n", encoding="utf-8", newline="\n")
    print(f"{path}: {len(lines)} lines -> {len(out)} lines")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
