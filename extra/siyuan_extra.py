import json
import mimetypes
import os
import threading
import time
from pathlib import Path
from typing import Any, Dict, Iterable, Optional

import boto3
import requests
from botocore.config import Config
from flask import Flask, jsonify, request


SIYUAN_URL = os.getenv("SIYUAN_URL", "http://siyuan:6806").rstrip("/")
SIYUAN_TOKEN = os.getenv("SIYUAN_TOKEN", "")
DATA_DIR = Path(os.getenv("EXTRA_DATA_DIR", "/data"))
ASSETS_DIR = Path(os.getenv("ASSETS_DIR", "/siyuan/workspace/data/assets"))

S3_ENDPOINT_URL = os.getenv("S3_ENDPOINT_URL") or None
S3_BUCKET = os.getenv("S3_BUCKET", "")
S3_REGION = os.getenv("S3_REGION", "auto")
S3_ACCESS_KEY_ID = os.getenv("S3_ACCESS_KEY_ID", "")
S3_SECRET_ACCESS_KEY = os.getenv("S3_SECRET_ACCESS_KEY", "")
S3_PREFIX = os.getenv("S3_PREFIX", "siyuan-assets").strip("/")
S3_FORCE_PATH_STYLE = os.getenv("S3_FORCE_PATH_STYLE", "true").lower() in ("1", "true", "yes")
ASSET_PUBLIC_BASE = os.getenv("ASSET_PUBLIC_BASE", "").rstrip("/")

WECHAT_WEBHOOK_URL = os.getenv("WECHAT_WEBHOOK_URL", "")
WECHAT_WEBHOOK_TYPE = os.getenv("WECHAT_WEBHOOK_TYPE", "work_wechat")

REMINDER_INTERVAL_SECONDS = int(os.getenv("REMINDER_INTERVAL_SECONDS", "60"))
ASSET_SCAN_INTERVAL_SECONDS = int(os.getenv("ASSET_SCAN_INTERVAL_SECONDS", "300"))

STATE_FILE = DATA_DIR / "asset-state.json"

app = Flask(__name__)
state_lock = threading.Lock()


def siyuan_headers() -> Dict[str, str]:
    headers = {"Content-Type": "application/json"}
    if SIYUAN_TOKEN:
        headers["Authorization"] = f"Token {SIYUAN_TOKEN}"
    return headers


def siyuan_post(path: str, payload: Dict[str, Any]) -> Dict[str, Any]:
    url = f"{SIYUAN_URL}{path}"
    response = requests.post(url, headers=siyuan_headers(), json=payload, timeout=30)
    response.raise_for_status()
    data = response.json()
    if data.get("code", 0) not in (0, None):
        raise RuntimeError(f"SiYuan API {path} failed: {data.get('msg')}")
    return data


def load_state() -> Dict[str, Any]:
    DATA_DIR.mkdir(parents=True, exist_ok=True)
    if not STATE_FILE.exists():
        return {"assets": {}}
    with STATE_FILE.open("r", encoding="utf-8") as f:
        return json.load(f)


def save_state(state: Dict[str, Any]) -> None:
    DATA_DIR.mkdir(parents=True, exist_ok=True)
    tmp = STATE_FILE.with_suffix(".tmp")
    with tmp.open("w", encoding="utf-8") as f:
        json.dump(state, f, ensure_ascii=False, indent=2, sort_keys=True)
    tmp.replace(STATE_FILE)


def s3_client():
    if not S3_BUCKET:
        return None
    addressing_style = "path" if S3_FORCE_PATH_STYLE else "virtual"
    return boto3.client(
        "s3",
        endpoint_url=S3_ENDPOINT_URL,
        region_name=S3_REGION,
        aws_access_key_id=S3_ACCESS_KEY_ID or None,
        aws_secret_access_key=S3_SECRET_ACCESS_KEY or None,
        config=Config(s3={"addressing_style": addressing_style}),
    )


def asset_key(relative_path: str) -> str:
    relative_path = relative_path.replace("\\", "/").lstrip("/")
    if S3_PREFIX:
        return f"{S3_PREFIX}/{relative_path}"
    return relative_path


def public_url_for(key: str) -> str:
    if ASSET_PUBLIC_BASE:
        return f"{ASSET_PUBLIC_BASE}/{key}"
    if S3_ENDPOINT_URL:
        return f"{S3_ENDPOINT_URL.rstrip('/')}/{S3_BUCKET}/{key}"
    return f"https://{S3_BUCKET}.s3.{S3_REGION}.amazonaws.com/{key}"


def iter_asset_files() -> Iterable[Path]:
    if not ASSETS_DIR.exists():
        return []
    return (
        path
        for path in ASSETS_DIR.rglob("*")
        if path.is_file() and not any(part.startswith(".") for part in path.relative_to(ASSETS_DIR).parts)
    )


def upload_asset(path: Path, client) -> Optional[Dict[str, Any]]:
    stat = path.stat()
    rel = path.relative_to(ASSETS_DIR).as_posix()
    key = asset_key(rel)
    content_type = mimetypes.guess_type(path.name)[0] or "application/octet-stream"
    extra_args = {"ContentType": content_type}
    client.upload_file(str(path), S3_BUCKET, key, ExtraArgs=extra_args)
    return {
        "path": rel,
        "key": key,
        "url": public_url_for(key),
        "size": stat.st_size,
        "mtime": stat.st_mtime,
        "uploadedAt": int(time.time()),
    }


def sync_assets_once() -> Dict[str, Any]:
    client = s3_client()
    if client is None:
        return {"enabled": False, "uploaded": 0, "skipped": 0}

    uploaded = []
    skipped = 0
    with state_lock:
        state = load_state()
        assets = state.setdefault("assets", {})
        for path in iter_asset_files():
            stat = path.stat()
            rel = path.relative_to(ASSETS_DIR).as_posix()
            old = assets.get(rel)
            if old and old.get("size") == stat.st_size and old.get("mtime") == stat.st_mtime:
                skipped += 1
                continue
            info = upload_asset(path, client)
            assets[rel] = info
            uploaded.append(info)
        save_state(state)
    return {"enabled": True, "uploaded": len(uploaded), "skipped": skipped, "items": uploaded}


def send_wechat_message(text: str) -> None:
    if not WECHAT_WEBHOOK_URL:
        return
    if WECHAT_WEBHOOK_TYPE == "work_wechat":
        payload = {"msgtype": "text", "text": {"content": text}}
    else:
        payload = {"text": text}
    response = requests.post(WECHAT_WEBHOOK_URL, json=payload, timeout=20)
    response.raise_for_status()


def due_reminders() -> Iterable[Dict[str, Any]]:
    now = time.strftime("%Y%m%d%H%M%S", time.localtime())
    stmt = (
        "SELECT a.block_id, a.value, b.content, b.hpath "
        "FROM attributes AS a LEFT JOIN blocks AS b ON a.block_id = b.id "
        "WHERE a.name = 'custom-reminder-wechat' AND a.value <= "
        f"'{now}' ORDER BY a.value ASC LIMIT 50"
    )
    data = siyuan_post("/api/query/sql", {"stmt": stmt})
    return data.get("data") or []


def process_reminders_once() -> Dict[str, Any]:
    sent = 0
    failed = 0
    for item in due_reminders():
        block_id = item.get("block_id")
        if not block_id:
            continue
        title = item.get("hpath") or item.get("content") or block_id
        timed = item.get("value") or ""
        try:
            send_wechat_message(f"SiYuan reminder {timed}\n{title}\nBlock: {block_id}")
            siyuan_post("/api/attr/setBlockAttrs", {"id": block_id, "attrs": {"custom-reminder-wechat": None}})
            sent += 1
        except Exception as exc:
            failed += 1
            print(f"reminder failed for {block_id}: {exc}", flush=True)
    return {"sent": sent, "failed": failed}


def loop(name: str, interval: int, fn) -> None:
    while True:
        try:
            result = fn()
            if result:
                print(f"{name}: {result}", flush=True)
        except Exception as exc:
            print(f"{name} failed: {exc}", flush=True)
        time.sleep(max(5, interval))


@app.get("/healthz")
def healthz():
    return jsonify({"ok": True})


@app.post("/clip")
@app.post("/collect")
def clip():
    payload = request.get_json(force=True) or {}
    title = payload.get("title", "")
    md = payload.get("md") or payload.get("markdown") or payload.get("content") or ""
    url = payload.get("url", "")
    data = siyuan_post("/api/inbox/addShorthand", {"title": title, "md": md, "url": url})
    return jsonify(data)


@app.post("/assets/sync")
def assets_sync():
    return jsonify(sync_assets_once())


@app.get("/assets/state")
def assets_state():
    with state_lock:
        return jsonify(load_state())


@app.post("/reminders/run")
def reminders_run():
    return jsonify(process_reminders_once())


if __name__ == "__main__":
    DATA_DIR.mkdir(parents=True, exist_ok=True)
    if REMINDER_INTERVAL_SECONDS > 0:
        threading.Thread(target=loop, args=("reminders", REMINDER_INTERVAL_SECONDS, process_reminders_once), daemon=True).start()
    if ASSET_SCAN_INTERVAL_SECONDS > 0:
        threading.Thread(target=loop, args=("assets", ASSET_SCAN_INTERVAL_SECONDS, sync_assets_once), daemon=True).start()
    app.run(host="0.0.0.0", port=int(os.getenv("PORT", "6810")))
