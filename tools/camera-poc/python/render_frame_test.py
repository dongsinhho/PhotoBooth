#!/usr/bin/env python3
"""Config-driven, disposable frame-render POC. Not production PhotoBooth code."""
from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any

import cv2
import numpy as np

DEFAULT_CONFIG = Path(__file__).resolve().parents[1] / "sample-frame.json"
MAX_CANVAS_DIMENSION = 4000
MAX_INPUT_SLOTS = 12


class ConfigError(ValueError):
    pass


def parse_color(value: Any, label: str = "color") -> tuple[int, int, int]:
    if not isinstance(value, str) or len(value) != 7 or not value.startswith("#"):
        raise ConfigError(f"{label} must be a hex color such as #F5F5F5")
    try:
        red, green, blue = (int(value[i:i + 2], 16) for i in (1, 3, 5))
    except ValueError as exc:
        raise ConfigError(f"{label} contains invalid hex digits") from exc
    return blue, green, red  # OpenCV uses BGR.


def require_int(value: Any, label: str, minimum: int, maximum: int) -> int:
    if isinstance(value, bool) or not isinstance(value, int) or not minimum <= value <= maximum:
        raise ConfigError(f"{label} must be an integer between {minimum} and {maximum}")
    return value


def validate_config(config: Any) -> dict[str, Any]:
    if not isinstance(config, dict):
        raise ConfigError("frame config must be a JSON object")
    if config.get("schema_version") != 1:
        raise ConfigError("schema_version must be 1")

    canvas = config.get("canvas")
    if not isinstance(canvas, dict):
        raise ConfigError("canvas must be an object")
    width = require_int(canvas.get("width"), "canvas.width", 64, MAX_CANVAS_DIMENSION)
    height = require_int(canvas.get("height"), "canvas.height", 64, MAX_CANVAS_DIMENSION)
    parse_color(canvas.get("background"), "canvas.background")

    border = canvas.get("border", {})
    if not isinstance(border, dict):
        raise ConfigError("canvas.border must be an object")
    if border:
        parse_color(border.get("color"), "canvas.border.color")
        thickness = require_int(border.get("thickness"), "canvas.border.thickness", 1, 100)
        inset = require_int(border.get("inset", 0), "canvas.border.inset", 0, min(width, height) // 2)
        if 2 * inset + thickness >= min(width, height):
            raise ConfigError("canvas border does not fit the canvas")

    slots = config.get("slots")
    if not isinstance(slots, list) or not 1 <= len(slots) <= MAX_INPUT_SLOTS:
        raise ConfigError(f"slots must contain 1 to {MAX_INPUT_SLOTS} items")
    for index, slot in enumerate(slots):
        prefix = f"slots[{index}]"
        if not isinstance(slot, dict):
            raise ConfigError(f"{prefix} must be an object")
        if not isinstance(slot.get("id"), str) or not slot["id"].strip():
            raise ConfigError(f"{prefix}.id must be a non-empty string")
        x = require_int(slot.get("x"), f"{prefix}.x", 0, width - 1)
        y = require_int(slot.get("y"), f"{prefix}.y", 0, height - 1)
        slot_w = require_int(slot.get("width"), f"{prefix}.width", 1, width)
        slot_h = require_int(slot.get("height"), f"{prefix}.height", 1, height)
        if x + slot_w > width or y + slot_h > height:
            raise ConfigError(f"{prefix} extends outside the canvas")
        if slot.get("fit", "cover") not in ("cover", "contain"):
            raise ConfigError(f"{prefix}.fit must be 'cover' or 'contain'")
        if "background" in slot:
            parse_color(slot["background"], f"{prefix}.background")
        slot_border = slot.get("border", {})
        if not isinstance(slot_border, dict):
            raise ConfigError(f"{prefix}.border must be an object")
        if slot_border:
            parse_color(slot_border.get("color"), f"{prefix}.border.color")
            require_int(slot_border.get("thickness"), f"{prefix}.border.thickness", 1, 50)

    texts = config.get("text", [])
    if not isinstance(texts, list) or len(texts) > 40:
        raise ConfigError("text must be a list with at most 40 items")
    for index, item in enumerate(texts):
        prefix = f"text[{index}]"
        if not isinstance(item, dict) or not isinstance(item.get("value"), str):
            raise ConfigError(f"{prefix}.value must be a string")
        require_int(item.get("x"), f"{prefix}.x", 0, width - 1)
        require_int(item.get("y"), f"{prefix}.y", 0, height - 1)
        scale = item.get("scale")
        if isinstance(scale, bool) or not isinstance(scale, (int, float)) or not 0.1 <= scale <= 5:
            raise ConfigError(f"{prefix}.scale must be between 0.1 and 5")
        parse_color(item.get("color"), f"{prefix}.color")
        require_int(item.get("thickness", 1), f"{prefix}.thickness", 1, 20)

    quality = require_int(config.get("jpeg_quality", 92), "jpeg_quality", 1, 100)
    validated = dict(config)
    validated["canvas"] = dict(canvas, width=width, height=height)
    validated["slots"] = slots
    validated["text"] = texts
    validated["jpeg_quality"] = quality
    return validated


def fit_image(source: np.ndarray, width: int, height: int, mode: str, background: tuple[int, int, int]) -> np.ndarray:
    if source is None or source.size == 0 or source.ndim != 3 or source.shape[2] != 3:
        raise ValueError("source image must be a non-empty 3-channel image")
    src_h, src_w = source.shape[:2]
    if mode == "cover":
        scale = max(width / src_w, height / src_h)
    elif mode == "contain":
        scale = min(width / src_w, height / src_h)
    else:
        raise ConfigError("fit must be 'cover' or 'contain'")
    interpolation = cv2.INTER_AREA if scale < 1 else cv2.INTER_LINEAR
    resized = cv2.resize(source, (max(1, round(src_w * scale)), max(1, round(src_h * scale))), interpolation=interpolation)
    resized_h, resized_w = resized.shape[:2]

    if mode == "cover":
        crop_x = max(0, (resized_w - width) // 2)
        crop_y = max(0, (resized_h - height) // 2)
        output = resized[crop_y:crop_y + height, crop_x:crop_x + width]
        if output.shape[:2] != (height, width):
            raise ValueError("center crop did not match slot dimensions")
        return output

    output = np.full((height, width, 3), background, dtype=np.uint8)
    x = (width - resized_w) // 2
    y = (height - resized_h) // 2
    output[y:y + resized_h, x:x + resized_w] = resized
    return output


def render(sources: list[np.ndarray], config: dict[str, Any]) -> np.ndarray:
    cfg = validate_config(config)
    if len(sources) != len(cfg["slots"]):
        raise ConfigError(f"received {len(sources)} image(s), but frame has {len(cfg['slots'])} slot(s)")

    canvas_cfg = cfg["canvas"]
    canvas_w, canvas_h = canvas_cfg["width"], canvas_cfg["height"]
    canvas = np.full((canvas_h, canvas_w, 3), parse_color(canvas_cfg["background"]), dtype=np.uint8)

    for source, slot in zip(sources, cfg["slots"]):
        x, y, width, height = slot["x"], slot["y"], slot["width"], slot["height"]
        background = parse_color(slot.get("background", canvas_cfg["background"]), f"slots[{slot['id']}].background")
        photo = fit_image(source, width, height, slot.get("fit", "cover"), background)
        canvas[y:y + height, x:x + width] = photo
        border = slot.get("border", {})
        if border:
            color = parse_color(border["color"], f"slots[{slot['id']}].border.color")
            thickness = border["thickness"]
            cv2.rectangle(canvas, (x, y), (x + width - 1, y + height - 1), color, thickness)

    border = canvas_cfg.get("border", {})
    if border:
        inset, thickness = border.get("inset", 0), border["thickness"]
        color = parse_color(border["color"], "canvas.border.color")
        cv2.rectangle(canvas, (inset, inset), (canvas_w - inset - 1, canvas_h - inset - 1), color, thickness)

    for item in cfg["text"]:
        cv2.putText(
            canvas,
            item["value"],
            (item["x"], item["y"]),
            cv2.FONT_HERSHEY_SIMPLEX,
            float(item["scale"]),
            parse_color(item["color"], "text.color"),
            item.get("thickness", 1),
            cv2.LINE_AA,
        )
    return canvas


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input", type=Path, action="append", required=True, help="image for each slot, in slot order; repeat for multi-photo frames")
    parser.add_argument("--config", type=Path, default=DEFAULT_CONFIG, help=f"JSON frame config (default: {DEFAULT_CONFIG})")
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()

    try:
        with args.config.open("r", encoding="utf-8-sig") as handle:
            config = json.load(handle)
        config = validate_config(config)
        sources: list[np.ndarray] = []
        for path in args.input:
            source = cv2.imread(str(path), cv2.IMREAD_COLOR)
            if source is None or not source.size:
                raise ValueError(f"could not decode input image: {path}")
            sources.append(source)
        output = render(sources, config)
        args.output.parent.mkdir(parents=True, exist_ok=True)
        if not cv2.imwrite(str(args.output), output, [cv2.IMWRITE_JPEG_QUALITY, config["jpeg_quality"]]):
            raise OSError(f"unable to write rendered output to {args.output}")
        height, width = output.shape[:2]
        print(f"PASS render slots={len(sources)} canvas={width}x{height} output={args.output}")
        print("This disposable output may contain webcam content. Keep it local and delete it after verification.")
        return 0
    except (OSError, json.JSONDecodeError, ConfigError, ValueError) as exc:
        print(f"FAIL render: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
