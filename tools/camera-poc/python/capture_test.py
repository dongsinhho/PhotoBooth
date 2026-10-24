#!/usr/bin/env python3
"""Disposable UVC webcam POC; not production PhotoBooth code."""
from __future__ import annotations
import argparse
import sys
import time
from pathlib import Path

try:
    import cv2
except ImportError:
    print("Missing dependency: install opencv-python in the POC virtual environment.", file=sys.stderr)
    raise SystemExit(2)

BACKENDS = {
    "auto": cv2.CAP_ANY,
    "msmf": getattr(cv2, "CAP_MSMF", cv2.CAP_ANY),
    "dshow": getattr(cv2, "CAP_DSHOW", cv2.CAP_ANY),
}

def open_camera(index: int, backend: str):
    cap = cv2.VideoCapture(index, BACKENDS[backend])
    if not cap.isOpened():
        cap.release()
        return None
    cap.set(cv2.CAP_PROP_FRAME_WIDTH, 1280)
    cap.set(cv2.CAP_PROP_FRAME_HEIGHT, 720)
    return cap

def probe(index: int, backend: str, count: int) -> bool:
    cap = open_camera(index, backend)
    if cap is None:
        return False
    good, shape = 0, None
    started = time.perf_counter()
    try:
        for _ in range(count):
            ok, frame = cap.read()
            if ok and frame is not None and frame.size:
                good += 1
                shape = frame.shape
    finally:
        cap.release()
    elapsed = time.perf_counter() - started
    if good:
        height, width = shape[:2]
        print(f"PASS index={index} backend={backend} frames={good}/{count} last_frame={width}x{height} elapsed={elapsed:.2f}s")
        return good == count
    print(f"FAIL index={index} backend={backend}: no frames read")
    return False

def capture_one(index: int, backend: str, count: int, output: Path) -> int:
    cap = open_camera(index, backend)
    if cap is None:
        print(f"FAIL index={index} backend={backend}: unable to open")
        return 1
    good, frame = 0, None
    started = time.perf_counter()
    try:
        for _ in range(count):
            ok, current = cap.read()
            if not ok or current is None or not current.size:
                print(f"FAIL: read failed after {good} valid frames")
                return 1
            good += 1
            frame = current
    finally:
        cap.release()
    output.parent.mkdir(parents=True, exist_ok=True)
    if frame is None or not cv2.imwrite(str(output), frame):
        print(f"FAIL: unable to write image to {output}")
        return 1
    height, width = frame.shape[:2]
    print(f"PASS index={index} backend={backend} frames={good} saved={output} dimensions={width}x{height} elapsed={time.perf_counter()-started:.2f}s")
    print("This image may include people or private surroundings; it is saved locally only. Delete it after verification.")
    return 0

def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--backend", choices=BACKENDS, default="dshow")
    parser.add_argument("--scan", action="store_true", help="probe indexes 0 through 7 without saving images")
    parser.add_argument("--index", type=int)
    parser.add_argument("--frames", type=int, default=50)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    if not 1 <= args.frames <= 10000:
        parser.error("--frames must be between 1 and 10000")
    if args.scan == (args.index is not None):
        parser.error("choose exactly one of --scan or --index")
    if args.index is not None and args.index < 0:
        parser.error("--index must be non-negative")
    if args.index is not None and args.output is None:
        parser.error("--output is required with --index")
    if args.scan:
        found = 0
        for index in range(8):
            if probe(index, args.backend, min(args.frames, 10)):
                found += 1
        print(f"Scan finished: {found} index(es) produced frames using backend={args.backend}")
        return 0 if found else 1
    return capture_one(args.index, args.backend, args.frames, args.output)

if __name__ == "__main__":
    raise SystemExit(main())
