# Google OAuth Integration

## Summary
Add Google OAuth 2.0 login and sign up to the helloworld project. This doc covers the initial integration design for issue https://github.com/yanqian/ai-helloworld/issues/3.

## Goals
- Allow users to register or log in with a Google account.
- Provide a Google OAuth entry point on the existing login page.
- Support refresh tokens to keep sessions active.
- Provide best-effort logout alignment between Google and the app session.

## Non-Goals
- Supporting other identity providers beyond Google.
- Replacing existing email/password authentication; Google auth will co-exist with it.
- Building advanced account management (e.g., MFA, recovery flows).

## Background / Context
The app already supports email-based login/sign up. Adding Google OAuth improves onboarding and reduces password friction. Google OAuth 2.0 will be used for authorization, with minimal scopes for identity and profile data.

## Proposed Design
- UI/UX
  - Add a "Continue with Google" button on the existing login/sign up page.
  - Redirect unauthenticated users to the login page when accessing protected content (current behavior remains).
- Auth flow
  - Implement OAuth 2.0 Authorization Code flow with PKCE and state validation.
  - Define endpoints:
    - `GET /auth/google/login` -> redirects to Google's authorization page.
    - `GET /auth/google/callback` -> exchanges code for tokens, creates or links user, starts session.
    - `POST /auth/logout` -> clears app session and optionally revokes Google refresh token.
  - Establish a first-party session (cookie or token) independent from Google; use it for app auth.
- Account mapping
  - Use Google `sub` (stable user ID) as provider identifier.
  - If a user already exists with the same email and no provider link, link the Google identity after verifying the email.
- Token handling
  - Store refresh tokens securely (encrypted at rest).
  - Rotate tokens when Google issues new ones; handle cases where refresh token is only returned on first consent.
  - Revoke refresh tokens on app logout where possible.

## Data / API Changes
- Database
  - Add OAuth identity table or fields on users:
    - `provider` (e.g., "google")
    - `provider_subject` (Google `sub`)
    - `refresh_token` (encrypted)
    - `provider_email` (for auditing)
  - Alternatively, a `user_identities` table for multi-provider support.
- API
  - New auth endpoints listed above.
  - Optional: token refresh endpoint if session is derived from OAuth tokens.

## Security / Privacy
- Use `state` to prevent CSRF and validate redirect URIs.
- Use PKCE; no implicit flow.
- Verify ID token signature, issuer, audience, and nonce where applicable.
- Restrict scopes to `openid`, `email`, and `profile`.
- Encrypt refresh tokens and restrict access in logs.
- Follow Google OAuth compliance for logout and user consent.

## Rollout Plan
- Implement behind a feature flag or configuration toggle.
- Deploy to staging for end-to-end verification.
- Gradual rollout to production after verification.

## Test Plan
- Unit tests for OAuth callback handling and identity mapping.
- Integration tests for login + callback + session creation.
- Manual UI verification for login, redirect, and logout flows.

## Open Questions
- Do we want to link Google accounts to existing users by email automatically or require confirmation?
- Google does not provide a reliable server-side signal for \"user logged out of Google\"; do we accept a local logout-only model or require periodic re-auth?
- Where should token encryption keys be stored (KMS vs app config)?
