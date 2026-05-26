# 05 — Rotate JWT_SECRET

**When to use:**
- Scheduled rotation (recommended every 90 days).
- Suspected leak — server compromise, env file in a public commit, ex-employee with credential history, etc.
**Time estimate:** 5 minutes operational; ~15-min user impact (all sessions invalidated).
**Risk level:** Medium — every active session is invalidated, forcing re-login.

## What happens on rotation

- Access tokens (HS256 JWT) are signed with `JWT_SECRET`. After rotation, all old tokens become unverifiable → 401 → frontend redirects to login.
- Refresh tokens are opaque random strings stored hashed in the `sessions` table — independent of `JWT_SECRET`. They will continue to work to mint new access tokens. **So users won't need to re-enter password if their refresh token is still valid.**
- Verdict: rotation is gentle (users may experience one redirect, not a full logout) unless you also revoke sessions.

## Routine rotation (no breach)

```bash
ssh deploy@<server>
cd /opt/hotel-booking

# 1. Generate a new strong secret.
NEW=$(openssl rand -base64 64 | tr -d '\n')
echo "NEW JWT_SECRET: $NEW"   # write down or paste into your password manager

# 2. Backup .env.prod, then update.
cp .env.prod .env.prod.bak.$(date -u +%FT%H%M%S)
sed -i.bak "s|^JWT_SECRET=.*|JWT_SECRET=$NEW|" .env.prod

# 3. Restart api + worker (no need to rebuild — config is loaded from env_file).
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d api worker

# 4. Verify.
curl -sSf https://api.${DOMAIN}/healthz
# Then load admin.${DOMAIN} in a browser — expect one redirect, then logged-in state restored.
```

## Emergency rotation (suspected leak)

In addition to the routine steps:

```sql
-- Revoke every active session so refresh-token chain ALSO breaks.
-- Every user will need to re-enter password.
UPDATE sessions SET revoked_at = NOW() WHERE revoked_at IS NULL;
```

Then send a customer notification:

```
Subject: Important: please sign in again

For security reasons we've rotated your session credentials. You'll be asked to sign in again next time you open the app. If you didn't request this and notice anything unusual, please reply to this email.
```

## If rotation breaks something

| Symptom | Likely cause | Fix |
|---|---|---|
| All API requests 500 after restart | API can't read `JWT_SECRET` from env (typo in .env.prod) | Verify with `docker compose exec api env \| grep JWT`. Restore from `.env.prod.bak.*`. |
| Some users 401-loop without ever staying logged in | Frontend keeps retrying the old access token | Hard-refresh the page or clear localStorage. Confirm refresh endpoint returns a new token. |
| Worker stops processing | Worker shares config; `JWT_SECRET` is required by the auth subsystem at startup | Same as above — verify env. |

## After-action (emergency only)

- [ ] Incident note in `docs/runbooks/incidents/YYYY-MM-DD-jwt-rotation.md` with timeline + scope.
- [ ] Audit `git log` for any commit that might have included the old secret.
- [ ] Review who had server access during the suspected leak window.
- [ ] Consider rotating other related secrets: `RESEND_API_KEY`, database password (entails an app restart + migration of the connection URL), MinIO root credentials.
