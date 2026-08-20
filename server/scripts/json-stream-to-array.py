#!/usr/bin/env python3
import json
import pathlib
import sys


def main() -> int:
    if len(sys.argv) != 3:
        print("usage: json-stream-to-array.py INPUT OUTPUT", file=sys.stderr)
        return 2
    raw = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
    decoder = json.JSONDecoder()
    offset = 0
    values = []
    while offset < len(raw):
        while offset < len(raw) and raw[offset].isspace():
            offset += 1
        if offset >= len(raw):
            break
        value, offset = decoder.raw_decode(raw, offset)
        values.append(value)
    pathlib.Path(sys.argv[2]).write_text(
        json.dumps(values, ensure_ascii=True, separators=(",", ":")) + "\n",
        encoding="utf-8",
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
