import importlib


siyuan_extra = importlib.import_module("siyuan_extra")


class FakeS3Client:
    def __init__(self):
        self.uploads = []

    def upload_file(self, filename, bucket, key, ExtraArgs=None):
        self.uploads.append(
            {
                "filename": filename,
                "bucket": bucket,
                "key": key,
                "extra_args": ExtraArgs or {},
            }
        )


def test_clip_forwards_payload_to_siyuan(monkeypatch):
    calls = []

    def fake_siyuan_post(path, payload):
        calls.append((path, payload))
        return {"code": 0, "data": {"oId": "123"}}

    monkeypatch.setattr(siyuan_extra, "siyuan_post", fake_siyuan_post)

    client = siyuan_extra.app.test_client()
    response = client.post(
        "/clip",
        json={"title": "Clip title", "markdown": "Clip body", "url": "https://example.com"},
    )

    assert response.status_code == 200
    assert response.get_json()["data"]["oId"] == "123"
    assert calls == [
        (
            "/api/inbox/addShorthand",
            {"title": "Clip title", "md": "Clip body", "url": "https://example.com"},
        )
    ]


def test_sync_assets_once_uploads_changed_files(tmp_path, monkeypatch):
    assets_dir = tmp_path / "assets"
    data_dir = tmp_path / "data"
    image_path = assets_dir / "note.png"
    image_path.parent.mkdir(parents=True)
    image_path.write_bytes(b"png")

    fake_client = FakeS3Client()
    monkeypatch.setattr(siyuan_extra, "ASSETS_DIR", assets_dir)
    monkeypatch.setattr(siyuan_extra, "DATA_DIR", data_dir)
    monkeypatch.setattr(siyuan_extra, "STATE_FILE", data_dir / "asset-state.json")
    monkeypatch.setattr(siyuan_extra, "S3_BUCKET", "bucket")
    monkeypatch.setattr(siyuan_extra, "S3_PREFIX", "prefix")
    monkeypatch.setattr(siyuan_extra, "ASSET_PUBLIC_BASE", "https://cdn.example.com")
    monkeypatch.setattr(siyuan_extra, "s3_client", lambda: fake_client)

    result = siyuan_extra.sync_assets_once()

    assert result["enabled"] is True
    assert result["uploaded"] == 1
    assert result["items"][0]["key"] == "prefix/note.png"
    assert result["items"][0]["url"] == "https://cdn.example.com/prefix/note.png"
    assert fake_client.uploads[0]["bucket"] == "bucket"
    assert fake_client.uploads[0]["key"] == "prefix/note.png"
    assert fake_client.uploads[0]["extra_args"]["ContentType"] == "image/png"

    second = siyuan_extra.sync_assets_once()
    assert second["uploaded"] == 0
    assert second["skipped"] == 1


def test_process_reminders_sends_message_and_clears_attr(monkeypatch):
    messages = []
    siyuan_calls = []

    monkeypatch.setattr(
        siyuan_extra,
        "due_reminders",
        lambda: [
            {
                "block_id": "20260101010101-abcdefg",
                "value": "20260627120000",
                "content": "Call back",
                "hpath": "/Inbox/Call back",
            }
        ],
    )
    monkeypatch.setattr(siyuan_extra, "send_wechat_message", messages.append)
    monkeypatch.setattr(
        siyuan_extra,
        "siyuan_post",
        lambda path, payload: siyuan_calls.append((path, payload)) or {"code": 0},
    )

    result = siyuan_extra.process_reminders_once()

    assert result == {"sent": 1, "failed": 0}
    assert "20260627120000" in messages[0]
    assert "/Inbox/Call back" in messages[0]
    assert siyuan_calls == [
        (
            "/api/attr/setBlockAttrs",
            {
                "id": "20260101010101-abcdefg",
                "attrs": {"custom-reminder-wechat": None},
            },
        )
    ]


def test_sync_assets_once_disabled_without_bucket(monkeypatch):
    monkeypatch.setattr(siyuan_extra, "S3_BUCKET", "")

    assert siyuan_extra.sync_assets_once() == {"enabled": False, "uploaded": 0, "skipped": 0}
