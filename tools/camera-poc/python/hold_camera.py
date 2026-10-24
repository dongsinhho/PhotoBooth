#!/usr/bin/env python3
"""Open a UVC camera for a bounded interval to test concurrent camera access."""
import argparse
import time
import cv2

BACKENDS = {
    "auto": cv2.CAP_ANY,
    "msmf": getattr(cv2, "CAP_MSMF", cv2.CAP_ANY),
    "dshow": getattr(cv2, "CAP_DSHOW", cv2.CAP_ANY),
}

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--index", type=int, default=0)
    parser.add_argument("--backend", choices=BACKENDS, default="msmf")
    parser.add_argument("--seconds", type=int, default=15)
    args = parser.parse_args()
    cap = cv2.VideoCapture(args.index, BACKENDS[args.backend])
    if not cap.isOpened():
        cap.release()
        print("HOLDER_OPEN_FAILED")
        return 1
    print(f"HOLDER_OPENED backend={args.backend}; holding camera for {args.seconds}s", flush=True)
    try:
        deadline = time.monotonic() + max(1, min(args.seconds, 60))
        while time.monotonic() < deadline:
            ok, frame = cap.read()
            if not ok or frame is None:
                print("HOLDER_READ_FAILED", flush=True)
                return 1
            time.sleep(0.05)
    finally:
        cap.release()
    print("HOLDER_RELEASED", flush=True)
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
