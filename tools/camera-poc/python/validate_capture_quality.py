#!/usr/bin/env python3
"""Measure PhotoBooth capture quality and flag gross darkness / first-shot exposure gaps.

This is a diagnostic gate, not a universal photographic-quality score. Run it only
on a static test scene with fixed camera position and lighting when comparing runs.
"""
from __future__ import annotations

import argparse
import json
import re
import statistics
import sys
from pathlib import Path
from typing import Any

import cv2


CAPTURE_RE = re.compile(r"^capture-(\d+)\.jpe?g$", re.IGNORECASE)


def discover_captures(inputs: list[Path]) -> list[Path]:
    found: set[Path] = set()
    for item in inputs:
        if item.is_file():
            if CAPTURE_RE.match(item.name):
                found.add(item.resolve())
            continue
        if item.is_dir():
            found.update(
                path.resolve()
                for path in item.rglob("*")
                if path.is_file() and CAPTURE_RE.match(path.name)
            )
    return sorted(found, key=lambda path: (str(path.parent).casefold(), shot_number(path), str(path).casefold()))


def shot_number(path: Path) -> int:
    match = CAPTURE_RE.match(path.name)
    return int(match.group(1)) if match else 0


def measure_capture(path: Path, expected_width: int, expected_height: int) -> dict[str, Any]:
    image = cv2.imread(str(path), cv2.IMREAD_COLOR)
    if image is None:
        raise ValueError(f"Unable to decode JPEG: {path}")
    height, width = image.shape[:2]
    gray = cv2.cvtColor(image, cv2.COLOR_BGR2GRAY)
    return {
        "path": str(path),
        "session_id": path.parent.parent.name,
        "shot": shot_number(path),
        "width": width,
        "height": height,
        "dimensions_ok": width == expected_width and height == expected_height,
        "mean_luma": round(float(gray.mean()), 3),
        "contrast_sd": round(float(gray.std()), 3),
        "laplacian_var": round(float(cv2.Laplacian(gray, cv2.CV_64F).var()), 3),
        "dark_pixel_pct": round(float((gray <= 15).mean() * 100), 3),
        "clipped_pct": round(float(((gray <= 5) | (gray >= 250)).mean() * 100), 3),
        "bytes": path.stat().st_size,
    }


def evaluate_session(
    captures: list[dict[str, Any]],
    min_mean_luma: float,
    max_first_shot_drop_pct: float,
) -> dict[str, Any]:
    ordered = sorted(captures, key=lambda row: int(row["shot"]))
    issues: list[str] = []
    if not ordered:
        return {"session_id": "", "status": "FAIL", "issues": ["no captures"], "captures": []}

    for row in ordered:
        if not row["dimensions_ok"]:
            issues.append(
                f"shot {row['shot']} has unexpected dimensions {row['width']}x{row['height']}"
            )
        if float(row["mean_luma"]) < min_mean_luma:
            issues.append(
                f"shot {row['shot']} is extremely dark (mean luminance "
                f"{row['mean_luma']} < {min_mean_luma})"
            )
        if int(row["bytes"]) <= 0:
            issues.append(f"shot {row['shot']} is empty")

    first = next((row for row in ordered if int(row["shot"]) == 1), None)
    later = [row for row in ordered if int(row["shot"]) >= 2]
    later_median = statistics.median(float(row["mean_luma"]) for row in later) if later else None
    first_drop_pct = None
    if first is not None and later_median is not None and later_median > 0:
        first_drop_pct = max(
            0.0,
            (later_median - float(first["mean_luma"])) / later_median * 100.0,
        )
        if first_drop_pct > max_first_shot_drop_pct:
            issues.append(
                f"shot 1 is {first_drop_pct:.2f}% darker than the median of later shots "
                f"(limit {max_first_shot_drop_pct:.2f}%)"
            )

    return {
        "session_id": str(ordered[0]["session_id"]),
        "status": "FAIL" if issues else "PASS",
        "issues": issues,
        "capture_count": len(ordered),
        "mean_luma_avg": round(statistics.mean(float(row["mean_luma"]) for row in ordered), 3),
        "later_shot_mean_luma_median": round(later_median, 3) if later_median is not None else None,
        "first_shot_drop_pct": round(first_drop_pct, 3) if first_drop_pct is not None else None,
        "captures": ordered,
    }


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("inputs", nargs="+", type=Path, help="capture JPEGs and/or directories to scan")
    parser.add_argument("--width", type=int, default=1280, help="expected capture width (default: 1280)")
    parser.add_argument("--height", type=int, default=720, help="expected capture height (default: 720)")
    parser.add_argument(
        "--min-mean-luma",
        type=float,
        default=20.0,
        help="gross-dark sentinel on 0-255 grayscale scale; diagnostic default 20, not a beauty threshold",
    )
    parser.add_argument(
        "--max-first-shot-drop-pct",
        type=float,
        default=10.0,
        help="maximum allowed first-shot luminance drop vs median of shots 2+ (default: 10%%)",
    )
    parser.add_argument("--json-out", type=Path, help="optional path for a machine-readable JSON report")
    args = parser.parse_args(argv)

    if args.width <= 0 or args.height <= 0:
        parser.error("--width and --height must be positive")
    if args.min_mean_luma < 0 or args.min_mean_luma > 255:
        parser.error("--min-mean-luma must be within 0..255")
    if args.max_first_shot_drop_pct < 0 or args.max_first_shot_drop_pct > 100:
        parser.error("--max-first-shot-drop-pct must be within 0..100")

    paths = discover_captures(args.inputs)
    if not paths:
        print("No capture-XX.jpg files found in supplied inputs.", file=sys.stderr)
        return 2

    groups: dict[Path, list[dict[str, Any]]] = {}
    failed_decode = False
    for path in paths:
        try:
            metric = measure_capture(path, args.width, args.height)
        except (OSError, ValueError) as exc:
            print(f"DECODE_ERROR: {exc}", file=sys.stderr)
            failed_decode = True
            continue
        groups.setdefault(path.parent, []).append(metric)

    sessions = [
        evaluate_session(rows, args.min_mean_luma, args.max_first_shot_drop_pct)
        for _, rows in sorted(groups.items(), key=lambda pair: str(pair[0]).casefold())
    ]
    report = {
        "status": "FAIL" if failed_decode or any(session["status"] != "PASS" for session in sessions) else "PASS",
        "capture_count": sum(int(session.get("capture_count", 0)) for session in sessions),
        "session_count": len(sessions),
        "policy": {
            "expected_dimensions": [args.width, args.height],
            "min_mean_luma": args.min_mean_luma,
            "max_first_shot_drop_pct": args.max_first_shot_drop_pct,
            "note": "Gross-dark and relative-consistency diagnostics only; not a universal photo-quality certification.",
        },
        "sessions": sessions,
    }
    print(json.dumps(report, indent=2))
    if args.json_out:
        args.json_out.parent.mkdir(parents=True, exist_ok=True)
        args.json_out.write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
        print(f"JSON_REPORT={args.json_out}")
    print(f"IMAGE_QUALITY_GATE={report['status']}")
    return 1 if report["status"] != "PASS" else 0


if __name__ == "__main__":
    raise SystemExit(main())
