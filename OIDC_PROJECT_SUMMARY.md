# Mattermost OIDC Integration (v11.6.0_oidc) - Project Summary

This document captures the current state of the custom OIDC (OpenID Connect) integration for Mattermost Team Edition, bypassing Enterprise Edition restrictions.

## 1. Technical Implementation

### Server-Side Bypasses
- **License Spoofing**: `server/channels/utils/license.go` modified to always return `OpenId: "true"` in the client license. This trickery enables OIDC settings in the webapp without a real Enterprise license.
- **Configuration Exposure**: `server/config/client.go` updated to expose OIDC and social login configurations to the frontend even when `license == nil`.

### OIDC Backend (`server/channels/api4/oidc.go`)
- **Path Hijacking**: Intercepts official paths:
  - `GET /oauth/openid/login`: Starts the OIDC flow.
  - `GET /signup/openid/complete`: Handles the provider callback.
- **Session Management**: Uses `c.App.AttachSessionCookies` for proper cookie/CSRF handling.
- **Absolute URLs**: Enforces absolute `redirect_uri` construction using `SiteURL` to satisfy Keycloak/OIDC provider requirements.

### Webapp UI Fixes
- **Admin Console**: `admin_definition.tsx` patched to remove `licensedForFeature('OpenId')` requirements.
- **Banner Neutralization**: `openid.tsx` and `openid_custom.tsx` discovery components now return `null` to hide "Contact Sales" and "Enterprise" upsell banners.
- **Routing**: Discovery routes in the System Console are actively hidden/redirected to prevent UI conflicts.

## 2. Versioning
- **Current Tag**: `v11.6.0_oidc`
- **Binary Version**: Identified as `11.6.0_oidc` in `server/public/model/version.go` and `webapp/package.json`.

## 3. Build Instructions
  [Refer](./server/build/DOCKER_BUILD.md)

## 4. Key Knowledge for Future Sessions
- **License Logic**: The server thinks it has OIDC because we tell it the *client* license has it. The backend flow is a custom "hijack" of the official endpoints.
- **Webapp Dependencies**: If the webapp build fails with `ERR_MODULE_NOT_FOUND`, ensure `chalk`, `concurrently`, and `blessed` are installed in the `webapp/` directory.
- **Architecture**: The `Dockerfile.custom` uses a multi-stage approach (Node 24 -> Go 1.25 -> Ubuntu Noble) and runs as the `mattermost` user (UID 2000).
