# films-go-optimisation

HTTP photo-processor for **Google Cloud Run** (Go).

Pipeline: **decode → EXIF rotate → resize → flatten alpha → blurhash → AVIF → Vercel Blob (private) → optional webhook to next-films**.

Does **not** write to Postgres / Prisma. DB ownership stays in next-films.

AVIF encode uses [`github.com/gen2brain/avif`](https://github.com/gen2brain/avif) (WASM/wazero, `CGO_ENABLED=0`) — same approach as `a-golang-quiz-youth-stolin`.

## Endpoints

| Method | Path | Auth |
| --- | --- | --- |
| `GET` | `/healthz` | none |
| `POST` | `/v1/process` | Bearer service token **or** short-lived job JWT |
| `POST` | `/v1/process-batch` | Bearer service token only |

### `POST /v1/process` (multipart)

Fields: `file`, `kind` (`poster` \| `portrait` \| `frame`), optional `entityId`, `movieId` (required for `frame`), `jobId`, `webhookUrl`.

If `jobId` + webhook URL (form or `DEFAULT_WEBHOOK_URL`) are set → after Blob put, service POSTs the same JSON to the webhook (HMAC + retries).

Job JWT (from admin UI): claims override `kind` / `entityId` / `movieId` / `jobId`.

### `POST /v1/process-batch` (multipart)

Fields: `files` (or repeated `file`), `kind`, `movieId` for frames, optional `webhookUrl`. Max 20 files. Processes with concurrency 4; webhook per item when URL is set.

### Success JSON

```json
{
  "ok": true,
  "jobId": "...",
  "entityId": "...",
  "kind": "poster",
  "movieId": null,
  "url": "https://….private.blob.vercel-storage.com/posters/….avif",
  "blurhash": "…",
  "width": 800,
  "height": 1200,
  "contentType": "image/avif",
  "bytes": 48210
}
```

Blob paths (compatible with next-films):

- poster → `posters/{entityId}.avif`
- portrait → `people/{entityId}.avif`
- frame → `frame-guess/{movieId}/{entityId}.avif`

## Env (Cloud Run)

| Variable | Required | Description |
| --- | --- | --- |
| `PHOTO_PROCESSOR_TOKEN` | one of token/JWT | Bearer for Next → Cloud Run |
| `PHOTO_JOB_JWT_SECRET` | one of token/JWT | HS256 secret for admin job JWTs |
| `BLOB_READ_WRITE_TOKEN` | yes | Same private Blob token as next-films |
| `PHOTO_WEBHOOK_SECRET` | for async | Bearer + HMAC key for outgoing webhook |
| `DEFAULT_WEBHOOK_URL` | no | e.g. `https://app.vercel.app/api/internal/photos/completed` |
| `PORT` | Cloud Run sets | Listen address |
| `AVIF_QUALITY` | no | default `38` |
| `AVIF_SPEED` | no | default `6` (gen2brain Speed) |
| `MAX_UPLOAD_BYTES` | no | default `2097152` (2 MiB) |
| `JOB_JWT_ISSUER` / `JOB_JWT_AUDIENCE` | no | optional JWT checks |

**Do not** set `POSTGRES_URL` here.

### next-films (integration)

| Variable | Description |
| --- | --- |
| `PHOTO_PROCESSOR_URL` | Cloud Run base URL |
| `PHOTO_PROCESSOR_TOKEN` | same as Go |
| `PHOTO_JOB_JWT_SECRET` | same as Go — mint tickets for admin |
| `PHOTO_WEBHOOK_SECRET` | verify `/api/internal/photos/completed` |
| `BLOB_READ_WRITE_TOKEN` | unchanged |

Job JWT claims (HS256): `jobId`, `kind`, `entityId`, `movieId?`, standard `exp` (≤5 min).

## Local run

```bash
set PHOTO_PROCESSOR_TOKEN=dev-token
set PHOTO_JOB_JWT_SECRET=dev-job-secret
set BLOB_READ_WRITE_TOKEN=vercel_blob_rw_…
set PHOTO_WEBHOOK_SECRET=webhook-secret
go run ./cmd/server
```

```bash
curl -sS -X POST "http://localhost:8080/v1/process" \
  -H "Authorization: Bearer $PHOTO_PROCESSOR_TOKEN" \
  -F "file=@poster.jpg;type=image/jpeg" \
  -F "kind=poster"
```

Batch:

```bash
curl -sS -X POST "http://localhost:8080/v1/process-batch" \
  -H "Authorization: Bearer $PHOTO_PROCESSOR_TOKEN" \
  -F "kind=frame" \
  -F "movieId=0193aaaaaaaaaaaaaaaaaaaaaaaa" \
  -F "files=@a.jpg" \
  -F "files=@b.jpg"
```

## Deploy (Cloud Run)

Thin Alpine Dockerfile, no CGo / libvips:

```bash
gcloud run deploy films-photo-processor \
  --source . \
  --region europe-west1 \
  --allow-unauthenticated \
  --memory 1Gi \
  --cpu 1 \
  --concurrency 4 \
  --timeout 120 \
  --set-secrets=PHOTO_PROCESSOR_TOKEN=photo-processor-token:latest,PHOTO_JOB_JWT_SECRET=photo-job-jwt:latest,BLOB_READ_WRITE_TOKEN=blob-rw:latest,PHOTO_WEBHOOK_SECRET=photo-webhook:latest \
  --set-env-vars=DEFAULT_WEBHOOK_URL=https://YOUR_APP.vercel.app/api/internal/photos/completed
```

Recommended: CPU always allocated for AVIF.

Auth is enforced in-app (Bearer / job JWT). Prefer Secret Manager over plain env for tokens.

## Webhook contract

`POST {webhookUrl}`

Headers:

- `Authorization: Bearer <PHOTO_WEBHOOK_SECRET>`
- `Content-Type: application/json`
- `X-Job-Id`, `X-Timestamp` (unix)
- `X-Signature`: `hex(HMAC-SHA256(secret, timestamp + "." + rawBody))`

Body: same as sync success/failure JSON. Next should be idempotent on `jobId`.

## Layout

```text
cmd/server/main.go
internal/config/
internal/auth/
internal/mime/
internal/imageproc/
internal/blob/
internal/webhook/
internal/api/
Dockerfile
```
