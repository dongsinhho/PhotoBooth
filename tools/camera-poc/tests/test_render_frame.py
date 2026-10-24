from __future__ import annotations
import sys
import unittest
from pathlib import Path

import numpy as np

# Make the adjacent POC renderer importable from any working directory.
PYTHON_DIR = Path(__file__).resolve().parents[1] / "python"
sys.path.insert(0, str(PYTHON_DIR))
from render_frame_test import ConfigError, fit_image, render, validate_config


def base_config():
    return {
        "schema_version": 1,
        "canvas": {"width": 200, "height": 300, "background": "#F5F5F5"},
        "slots": [
            {"id": "photo-1", "x": 20, "y": 40, "width": 160, "height": 90, "fit": "cover"}
        ],
        "text": [],
        "jpeg_quality": 92,
    }


class FrameConfigTests(unittest.TestCase):
    def test_valid_config(self):
        cfg = validate_config(base_config())
        self.assertEqual(cfg["canvas"]["width"], 200)
        self.assertEqual(len(cfg["slots"]), 1)

    def test_rejects_unknown_schema(self):
        cfg = base_config()
        cfg["schema_version"] = 2
        with self.assertRaises(ConfigError):
            validate_config(cfg)

    def test_rejects_slot_outside_canvas(self):
        cfg = base_config()
        cfg["slots"][0]["x"] = 100
        with self.assertRaises(ConfigError):
            validate_config(cfg)

    def test_rejects_bad_color(self):
        cfg = base_config()
        cfg["canvas"]["background"] = "red"
        with self.assertRaises(ConfigError):
            validate_config(cfg)

    def test_rejects_mismatched_inputs_and_slots(self):
        source = np.full((50, 100, 3), 120, dtype=np.uint8)
        cfg = base_config()
        cfg["slots"].append({"id": "photo-2", "x": 20, "y": 150, "width": 160, "height": 90})
        with self.assertRaises(ConfigError):
            render([source], cfg)

    def test_cover_returns_exact_slot_dimensions(self):
        source = np.full((40, 100, 3), 120, dtype=np.uint8)
        result = fit_image(source, 40, 60, "cover", (255, 255, 255))
        self.assertEqual(result.shape, (60, 40, 3))

    def test_contain_returns_exact_slot_dimensions_and_padding(self):
        source = np.full((40, 100, 3), 100, dtype=np.uint8)
        result = fit_image(source, 100, 100, "contain", (255, 255, 255))
        self.assertEqual(result.shape, (100, 100, 3))
        self.assertTrue(np.all(result[0] == np.array([255, 255, 255])))

    def test_render_output_dimensions_and_slot_count(self):
        source = np.full((720, 1280, 3), (30, 90, 160), dtype=np.uint8)
        result = render([source], base_config())
        self.assertEqual(result.shape, (300, 200, 3))

    def test_render_three_photo_strip_slots(self):
        cfg = {
            "schema_version": 1,
            "canvas": {"width": 1200, "height": 1800, "background": "#F7F3F7"},
            "slots": [
                {"id": "photo-1", "x": 200, "y": 230, "width": 800, "height": 450, "fit": "cover"},
                {"id": "photo-2", "x": 200, "y": 715, "width": 800, "height": 450, "fit": "cover"},
                {"id": "photo-3", "x": 200, "y": 1200, "width": 800, "height": 450, "fit": "cover"},
            ],
            "text": [],
            "jpeg_quality": 92,
        }
        images = [
            np.full((720, 1280, 3), (30, 90, 160), dtype=np.uint8),
            np.full((720, 1280, 3), (60, 120, 180), dtype=np.uint8),
            np.full((720, 1280, 3), (90, 150, 210), dtype=np.uint8),
        ]
        result = render(images, cfg)
        self.assertEqual(result.shape, (1800, 1200, 3))
        self.assertEqual(len(validate_config(cfg)["slots"]), 3)

    def test_rejects_boolean_width(self):
        cfg = base_config()
        cfg["canvas"]["width"] = True
        with self.assertRaises(ConfigError):
            validate_config(cfg)


if __name__ == "__main__":
    unittest.main()
