# OIDC Patch: Keeping Up with Upstream Mattermost

## Overview

This patch adds generic OIDC (OpenID Connect) authentication support to Mattermost.

Upstream Mattermost had OIDC/OpenID support in earlier versions but **removed it
from the free/Team Edition tier in v11**, restricting it to the paid Enterprise
tier behind a license check. This patch restores OIDC support for all tiers
without requiring an Enterprise license.

### Why this fork exists

- Mattermost v10 and earlier: OIDC was available in Team Edition
- Mattermost v11+: OIDC is paywalled behind Enterprise license
- This fork: OIDC works for all users via environment variables, no license needed

## Files Modified

| File | Type | Description |
|------|------|-------------|
| `server/channels/api4/oidc.go` | **New** | OIDC handler (start/complete flow) |
| `server/channels/api4/api.go` | Modified | One line added: `api.InitOIDC()` |
| `server/go.mod` / `server/go.sum` | Modified | Added `coreos/go-oidc/v3`, `golang.org/x/oauth2` |

## How to Sync with Upstream

### Step 1: Add upstream remote (one-time setup)

```sh
cd ~/github/mattermost
git remote add upstream https://github.com/mattermost/mattermost.git
```

### Step 2: Fetch and merge upstream

```sh
git fetch upstream
git checkout main
git merge upstream/master
```

### Step 3: Resolve conflicts

The OIDC patch touches only **one line** in an existing file (`api.go`).
Conflicts will only arise if upstream modifies the `Init()` function in
`server/channels/api4/api.go` around the area where `api.InitOIDC()` is called.

**Typical resolution:**
- Keep upstream's new `Init*()` calls
- Re-add `api.InitOIDC()` before the testing/404 handler block

```go
// Look for this pattern in api.go Init():
api.InitAgents()
api.InitProperties()
api.InitOIDC()           // <-- ensure this line exists

// If we allow testing then listen for manual testing URL hits
```

### Step 4: Update Go dependencies

```sh
cd server
go mod tidy
```

### Step 5: Verify

```sh
cd server
go build ./channels/api4/
```

## Why This Patch is Low-Maintenance

1. **`oidc.go` is a standalone file** — it doesn't modify any existing Mattermost code.
   Upstream changes won't conflict with it unless they restructure the `api4` package.

2. **Single-line change in `api.go`** — only `api.InitOIDC()` is added.
   This is the same pattern Mattermost uses for all its features, so it's
   unlikely to be affected by refactoring.

3. **No frontend changes** — the OIDC login button is added via Mattermost's
   existing external login provider mechanism (configured via env vars).

4. **Dependencies are minimal** — `coreos/go-oidc/v3` and `golang.org/x/oauth2`
   are standard, stable libraries.

5. **Independent of upstream's OIDC** — this patch does not touch or depend on
   upstream's Enterprise OIDC implementation. It's a separate code path, so
   upstream adding/removing/modifying their licensed OIDC won't affect this patch.

## Upstream License Gating (v11+)

Starting with v11, upstream gates OIDC behind `model.BuildEnterpriseReady` and
license checks. When syncing with upstream, be aware:

- **Do NOT** accept upstream changes that add license checks to our `oidc.go`
- Upstream's OIDC lives in `server/channels/app/openid.go` and related files —
  those are separate from our `api4/oidc.go`
- If upstream restructures authentication, verify our endpoints still register correctly

## Configuration

Set these environment variables on the Mattermost server:

```sh
OIDC_ISSUER=https://keycloak.example.com/realms/myrealm
OIDC_CLIENT_ID=mattermost
OIDC_CLIENT_SECRET=your-client-secret
OIDC_REDIRECT_URL=https://mattermost.example.com/api/v4/auth/oidc/complete
```

## Security Notes vs Jacobamv's Original Patch

| Issue | Jacobamv | This patch |
|-------|----------|-----------|
| State store | Plain `map`, no mutex, no expiry | `sync.Mutex`, auto-expiry (10 min), cleanup goroutine |
| State validation | Proceeds without error if missing | Returns 400 error |
| Session expiry math | Wrong (hours * seconds-in-day) | Correct (`time.Duration * time.Hour`) |
| Env var names | Hardcoded to Keycloak | Generic OIDC (works with any provider) |
| Login page | Removes email/password form | Keeps existing login form intact |
| `enableExternalSignup` guard | Removed | Kept |
| Debug code | `console.log("test brosskev")` | Removed |
| Branding changes | Removes "FREE EDITION" | No branding changes |

## Automated Sync (Optional)

Add to CI:

```yaml
sync-upstream:
  runs-on: ubuntu-latest
  schedule:
    - cron: '0 0 * * 1'  # Weekly on Monday
  steps:
    - uses: actions/checkout@v4
    - run: |
        git remote add upstream https://github.com/mattermost/mattermost.git || true
        git fetch upstream
        git merge upstream/master --no-edit || {
          echo "Merge conflict detected. Manual resolution needed."
          exit 1
        }
        cd server && go mod tidy && go build ./channels/api4/
```
