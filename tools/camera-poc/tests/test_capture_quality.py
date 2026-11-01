from __future__ import annotations

import sys
import tempfile
import unittest
from pathlib import Path

import cv2
import numpy as np

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "python"))
from validate_capture_quality import evaluate_session, measure_capture  # noqa: E402


def fake_row(shot: int, luminance: float, session_id: str = "session-1") -> dict[str, object]:
    return {
        "path": f"{session_id}/captures/capture-{shot:02d}.jpg",
        "session_id": session_id,
        "shot": shot,
        "width": 1280,
        "height": 720,
        "dimensions_ok": True,
        "mean_luma": luminance,
        "contrast_sd": 0.0,
        "laplacian_var": 0.0,
        "dark_pixel_pct": 0.0,
        "clipped_pct": 0.0,
        "bytes": 100,
    }


class CaptureQualityTests(unittest.TestCase):
    def test_reports_too_dark_session_as_failure(self) -> None:
        report = evaluate_session(
            [fake_row(1, 2.8), fake_row(2, 2.7), fake_row(3, 2.7)],
            min_mean_luma=20.0,
            max_first_shot_drop_pct=10.0,
        )
        self.assertEqual(report["status"], "FAIL")
        self.assertTrue(any("extremely dark" in item for item in report["issues"]))

    def test_fails_when_first_shot_is_darker_than_later_shots(self) -> None:
        report = evaluate_session(
            [fake_row(1, 80), fake_row(2, 120), fake_row(3, 122)],
            min_mean_luma=20.0,
            max_first_shot_drop_pct=10.0,
        )
        self.assertEqual(report["status"], "FAIL")
        self.assertAlmostEqual(report["first_shot_drop_pct"], 33.884, places=2)

    def test_passes_consistent_bright_enough_session(self) -> None:
        report = evaluate_session(
            [fake_row(1, 133.07), fake_row(2, 133.10), fake_row(3, 133.07)],
            min_mean_luma=20.0,
            max_first_shot_drop_pct=10.0,
        )
        self.assertEqual(report["status"], "PASS")
        self.assertEqual(report["issues"], [])

    def test_flags_wrong_dimensions(self) -> None:
        first = fake_row(1, 100)
        first["width"] = 640
        first["dimensions_ok"] = False
        report = evaluate_session(
            [first, fake_row(2, 100), fake_row(3, 100)],
            min_mean_luma=20.0,
            max_first_shot_drop_pct=10.0,
        )
        self.assertEqual(report["status"], "FAIL")
        self.assertTrue(any("unexpected dimensions" in item for item in report["issues"]))

    def test_measure_capture_reads_real_jpeg_and_records_metrics(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            captures = Path(directory) / "sessions" / "session-1" / "captures"
            captures.mkdir(parents=True)
            image = np.full((720, 1280, 3), 128, dtype=np.uint8)
            path = captures / "capture-01.jpg"
            self.assertTrue(cv2.imwrite(str(path), image))
            result = measure_capture(path, 1280, 720)
            self.assertEqual(result["width"], 1280)
            self.assertEqual(result["height"], 720)
            self.assertEqual(result["shot"], 1)
            self.assertGreater(result["bytes"], 0)
            self.assertAlmostEqual(result["mean_luma"], 128.0, delta=1.0)


if __name__ == "__main__":
    unittest.main()
