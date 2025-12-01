# Cloudflare R2 Setup (step-by-step)

This is a reference to wire R2 as the S3-compatible object store for Upload & Ask. Follow these steps in order; nothing here is Codex-specific.

## 1) Create bucket
1. Log in to Cloudflare dashboard → R2.
2. Create bucket (e.g., `upload-ask-prod`).
3. Note the **Account ID** and the bucket name; you’ll need both for the S3 endpoint.

## 2) Create API token (S3-compatible)
1. R2 → **Manage R2 API Tokens** → **Create API token**.
2. Choose template **Edit** (or **Read & Write**) for the bucket.
3. Restrict to the bucket you created.
4. Save the **Access Key ID** and **Secret Access Key** (shown once). Treat as secrets.

## 3) Endpoint + URL format
- S3 endpoint: `https://<accountid>.r2.cloudflarestorage.com`
- Bucket URL pattern: `https://<bucket>.<accountid>.r2.cloudflarestorage.com`
- For SDKs, use the endpoint and pass the bucket separately; no region required.

## 4) CORS (if browser direct upload)
If you later use presigned PUTs from the browser, set CORS on the bucket:
```
Allowed origins: https://<your-frontend-domain>
Allowed methods: PUT, GET, HEAD
Allowed headers: content-type, authorization, x-amz-acl
Expose headers: etag
Max age: 3600
```
For backend-proxied uploads, CORS is less critical because the browser only hits your API.

## 5) GitHub Actions secrets/vars
Add these to your repo’s Actions settings (no .env files):
- Secrets: `R2_ENDPOINT=https://<accountid>.r2.cloudflarestorage.com`, `R2_ACCESS_KEY=<AccessKeyId>`, `R2_SECRET_KEY=<SecretAccessKey>`
- Vars: `R2_BUCKET=<bucket-name>`
Map them to the backend config at deploy time (e.g., env injection or config file templating).

## 6) Backend configuration (Go service)
- S3 client config:
  - `endpoint = R2_ENDPOINT`
  - `region = auto` (or any string; R2 ignores but some SDKs require)
  - `credentials = R2_ACCESS_KEY / R2_SECRET_KEY`
  - `s3ForcePathStyle = true` (recommended for R2)
- Bucket: `R2_BUCKET`
- Object keys: `uploads/{userId}/{documentId}/{filename}`
- Test: from a shell with aws-cli v2+
  ```
  AWS_ACCESS_KEY_ID=$R2_ACCESS_KEY \
  AWS_SECRET_ACCESS_KEY=$R2_SECRET_KEY \
  aws --endpoint-url $R2_ENDPOINT s3 ls s3://$R2_BUCKET
  ```

## 7) Frontend note
If you add presigned uploads:
- Backend issues presigned PUT via S3 SDK against R2 endpoint.
- Frontend uses that URL with headers: `Content-Type` and `x-amz-acl: private` (if required).
- Keep file size/type allowlists aligned with backend validation.

## 8) Lifecycle rules (optional later)
When volume grows, add bucket rules:
- Expire old raw uploads after N days if you retain processed text elsewhere.
- Transition to colder storage tiers (if/when available) to save cost.

## 9) Troubleshooting
- 403 errors: check bucket binding on the token and path-style setting.
- Signature mismatch: ensure the endpoint URL is used in signing and `s3ForcePathStyle=true`.
- CORS errors: verify allowed origins/methods/headers on the bucket.
