#!/usr/bin/env python3
"""Validate temporary PhotoBooth JPEG artifacts and report coarse image metrics."""
from __future__ import annotations

import statistics
import sys
from pathlib import Path

import cv2


def main() -> int:
    paths = [Path(item) for item in sys.argv[1:]]
    if len(paths) == 1 and paths[0].is_dir():
        paths = sorted(paths[0].rglob("*.jpg"))
    if not paths:
        print("usage: verify_soak_artifacts.py <jpeg> [<jpeg> ...] OR <directory>", file=sys.stderr)
        return 2

    rows: list[dict[str, object]] = []
    for path in paths:
        image = cv2.imread(str(path))
        if image is None:
            print(f"UNREADABLE: {path}", file=sys.stderr)
            return 3
        height, width = image.shape[:2]
        capture = path.name.lower().startswith("capture-")
        expected = (1280, 720) if capture else (1200, 1800)
        gray = cv2.cvtColor(image, cv2.COLOR_BGR2GRAY)
        rows.append({
            "name": path.name,
            "capture": capture,
            "width": width,
            "height": height,
            "expected": expected,
            "ok": (width, height) == expected,
            "mean_luma": float(gray.mean()),
            "contrast_sd": float(gray.std()),
            "laplacian_var": float(cv2.Laplacian(gray, cv2.CV_64F).var()),
            "clipped_pct": float(((gray <= 5) | (gray >= 250)).mean() * 100),
        })

    bad = [row for row in rows if not row["ok"]]
    captures = [row for row in rows if row["capture"]]
    outputs = [row for row in rows if row["name"] in {"photo.jpg", "rendered.jpg"}]
    print(f"JPEG_COUNT={len(rows)}")
    print(f"CAPTURE_COUNT={len(captures)}")
    print(f"RENDERED_AND_LOCAL_OUTPUT_COUNT={len(outputs)}")
    print(f"BAD_DIMENSION_COUNT={len(bad)}")
    for row in bad:
        print(f"BAD_DIMENSION: {row['name']} got {row['width']}x{row['height']} expected {row['expected']}")
    for label in ("mean_luma", "contrast_sd", "laplacian_var", "clipped_pct"):
        values = [float(row[label]) for row in captures]
        if values:
            print(f"CAPTURE_{label.upper()}_MEAN={statistics.mean(values):.2f}")
            print(f"CAPTURE_{label.upper()}_MIN={min(values):.2f}")
            print(f"CAPTURE_{label.upper()}_MAX={max(values):.2f}")
    print("QUALITY_NOTE=brightness/contrast/sharpness are coarse diagnostics, not a visual quality certification.")
    return 1 if bad else 0


if __name__ == "__main__":
    raise SystemExit(main())
