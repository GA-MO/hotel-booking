// Package upload implements the image-upload pipeline for hotel and
// room-type photos. The browser obtains a presigned PUT URL from
// POST /v1/uploads/presign, PUTs the file directly to S3-compatible storage
// (MinIO in dev, Backblaze B2 in prod), then POSTs the resulting public URL
// to the existing photo CRUD endpoints in [hotel] / [roomtype]. The API
// process never proxies the bytes.
//
// # Object key layout
//
// Keys are deterministically derived from the authenticated identity and the
// upload kind:
//
//	{account_id}/{hotel_id}/{kind}/{uuid}.{ext}
//
// This guarantees:
//   - Cross-tenant isolation at the object-key level (an `account_id` prefix
//     is the unforgeable root). Even if a presigned URL leaks, the signer
//     produced it for a key under the caller's `account_id`.
//   - Predictable lifecycle for moderation/cleanup (delete by prefix).
//
// # SigV4 presigning (PUT)
//
// The [Signer] hand-rolls AWS SigV4 query-string presigning for `PUT`
// requests (Algorithm = `AWS4-HMAC-SHA256`, payload = `UNSIGNED-PAYLOAD`).
// Both MinIO and Backblaze B2 accept SigV4, so the same code path works in
// dev and prod. We avoid `aws-sdk-go-v2` to keep the build small (the SDK
// pulls ~50 transitive deps and ~30 MB of source).
//
// Reference: AWS "Signing AWS requests with Signature Version 4" docs and
// the canonical test vectors (`get-vanilla.req` etc.) reproduced in
// [signer_test.go].
//
// # Security invariants
//
//   - Presigned URLs are time-limited (default 15 minutes; 900 s expiry).
//   - Each URL is bound to a single `(method, bucket, object_key,
//     content_type, max_size)` tuple — the browser cannot retarget the URL.
//   - `content_type` is whitelisted server-side (image/jpeg, image/png,
//     image/webp). Anything else returns [ErrUnsupportedKind].
//   - `size_bytes` is capped at [MaxUploadBytes] (10 MiB).
//   - Object keys are server-generated UUIDs — clients never choose them.
//
// # imgproxy URL signing
//
// [Imgproxy.Sign] returns a signed delivery URL of the form:
//
//	{base}/{signature}/{processing_options}/{base64-encoded-source-url}
//
// where `signature` = `base64url(HMAC-SHA256(key, salt || path))`. The key
// and salt are hex-decoded from [config.Config.ImgproxyKey] and
// [config.Config.ImgproxySalt]. When either is empty, [Imgproxy.Sign] emits
// the `insecure` placeholder (matching imgproxy's dev mode).
//
// Reference: https://docs.imgproxy.net/usage/signing_url
package upload
