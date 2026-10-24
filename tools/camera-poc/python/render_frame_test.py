#!/usr/bin/env python3
"""Disposable single-photo frame rendering POC; not the production renderer."""
from __future__ import annotations
import argparse
import sys
from pathlib import Path
import cv2
import numpy as np

def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    source = cv2.imread(str(args.input), cv2.IMREAD_COLOR)
    if source is None or not source.size:
        print("FAIL: could not decode input image", file=sys.stderr)
        return 1

    # Deliberately simple reference output: a 2:3 canvas with one image slot
    # and a title/footer. It validates capture-file -> raster-render -> file only.
    canvas_w, canvas_h = 1200, 1800
    canvas = np.full((canvas_h, canvas_w, 3), (245, 245, 245), dtype=np.uint8)
    accent = (180, 60, 200)  # BGR accent for this disposable sample.
    cv2.rectangle(canvas, (24, 24), (canvas_w - 25, canvas_h - 25), accent, 12)
    cv2.putText(canvas, "PHOTOBOOTH", (80, 130), cv2.FONT_HERSHEY_SIMPLEX, 1.8, accent, 4, cv2.LINE_AA)
    cv2.putText(canvas, "SAMPLE FRAME", (82, 190), cv2.FONT_HERSHEY_SIMPLEX, 0.9, (70, 70, 70), 2, cv2.LINE_AA)

    slot_x, slot_y, slot_w, slot_h = 80, 280, 1040, 1120
    src_h, src_w = source.shape[:2]
    scale = max(slot_w / src_w, slot_h / src_h)
    resized = cv2.resize(source, (round(src_w * scale), round(src_h * scale)), interpolation=cv2.INTER_AREA if scale < 1 else cv2.INTER_LINEAR)
    resized_h, resized_w = resized.shape[:2]
    crop_x = max(0, (resized_w - slot_w) // 2)
    crop_y = max(0, (resized_h - slot_h) // 2)
    photo = resized[crop_y:crop_y + slot_h, crop_x:crop_x + slot_w]
    if photo.shape[:2] != (slot_h, slot_w):
        print("FAIL: crop did not match the requested photo slot", file=sys.stderr)
        return 1
    canvas[slot_y:slot_y + slot_h, slot_x:slot_x + slot_w] = photo
    cv2.rectangle(canvas, (slot_x - 4, slot_y - 4), (slot_x + slot_w + 3, slot_y + slot_h + 3), (30, 30, 30), 5)
    cv2.putText(canvas, "YOUR PHOTO MOMENT", (80, 1510), cv2.FONT_HERSHEY_SIMPLEX, 1.1, accent, 3, cv2.LINE_AA)
    cv2.putText(canvas, "TECHNICAL TEST - NOT A PRODUCTION TEMPLATE", (80, 1580), cv2.FONT_HERSHEY_SIMPLEX, 0.55, (60, 60, 60), 2, cv2.LINE_AA)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    if not cv2.imwrite(str(args.output), canvas, [cv2.IMWRITE_JPEG_QUALITY, 92]):
        print("FAIL: unable to write rendered output", file=sys.stderr)
        return 1
    print(f"PASS render input={src_w}x{src_h} output={canvas_w}x{canvas_h} file={args.output}")
    print("Note: this output contains the camera frame; saved locally only and should be deleted after verification.")
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
