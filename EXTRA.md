# SiYuan Unlock Extra

This fork keeps the built-in S3/WebDAV/Local sync implementation available for self-hosted Docker use and adds a small optional sidecar service.

## Services

- `siyuan`: patched SiYuan kernel and frontend.
- `siyuan-extra`: optional MVP helper for local inbox intake, asset image-bed upload, and webhook reminders.

## NAS Compose

If you already run `apkdv/siyuan-unlock` with workspace data at `/share/Container/siyuan/workspace`, use `docker-compose.nas.yml`. It keeps the same host workspace mount, so existing notebooks and assets continue to live in the same directory.

```bash
cp .env.example .env
docker compose -f docker-compose.nas.yml pull
docker compose -f docker-compose.nas.yml up -d
```

The default `.env` values target your existing NAS path `/share/Container/siyuan/workspace`, access auth code `siyuan1896`, and the GHCR images built from this branch. Fill `SIYUAN_TOKEN`, S3, and WeChat webhook values only when you want the sidecar features.

## Required SiYuan API Token

Set a SiYuan API token in `Settings - About`, then pass it to the sidecar as `SIYUAN_TOKEN`. The sidecar uses `Authorization: Token ...`.

## Local Inbox

The SiYuan inbox panel is backed by `data/storage/local-inbox.json`.

Add a clipping item:

```bash
curl -X POST http://nas:6810/clip \
  -H 'Content-Type: application/json' \
  -d '{"title":"Example","url":"https://example.com","md":"Captured note"}'
```

The sidecar forwards this to `/api/inbox/addShorthand`; the normal inbox panel can read, move, and delete it.

## Reminders

The WeChat reminder UI writes the local block attribute `custom-reminder-wechat`. The sidecar scans due attributes through `/api/query/sql` and sends a webhook message.

For Work WeChat robots:

```env
WECHAT_WEBHOOK_URL=https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=...
WECHAT_WEBHOOK_TYPE=work_wechat
```

After a reminder is sent, the sidecar clears `custom-reminder-wechat` using `/api/attr/setBlockAttrs`.

## Asset Image Bed

The sidecar scans `${SIYUAN_WORKSPACE}/data/assets` and uploads changed files to an S3-compatible bucket.

Typical MinIO/R2-style config:

```env
S3_ENDPOINT_URL=https://s3.example.com
S3_BUCKET=siyuan-assets
S3_REGION=auto
S3_ACCESS_KEY_ID=...
S3_SECRET_ACCESS_KEY=...
S3_PREFIX=siyuan-assets
S3_FORCE_PATH_STYLE=true
ASSET_PUBLIC_BASE=https://cdn.example.com
```

Trigger an immediate scan:

```bash
curl -X POST http://nas:6810/assets/sync
```

The upload manifest is stored in the sidecar data volume as `asset-state.json`.
