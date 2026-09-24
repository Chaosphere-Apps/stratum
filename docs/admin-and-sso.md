# Administration and SSO

Stratum starts in onboarding mode when no users exist. The first user created from the onboarding screen becomes the organization admin.

Admins can open the Admin Console from the top navigation to manage:

- Users, product roles, account status, and password-reset links.
- Access groups and direct or group grants for workspaces and designs.
- Local password sign-in and Okta SSO.
- The governed component catalog.
- PostgreSQL storage configuration and migration from stateless mode.
- The organization AI provider used by analysis and design chat.
- Telemetry and MCP capability settings.

Configuration writes return visible success or failure feedback. Stored secrets are redacted from subsequent API responses and are not sent back to the browser.

## AI provider

Stratum supports OpenAI, Anthropic, OpenRouter, Google AI Studio (Gemini), and custom OpenAI-compatible endpoints. Configure one provider for the organization, enter its model and API key, and verify it before enabling AI features. A saved configuration is presented as configured state rather than repopulating the secret form.

Custom provider URLs must use HTTPS and cannot target private, loopback, link-local, or otherwise unsafe network addresses unless the deployment explicitly enables private provider URLs. Keep that override disabled for managed providers and internet-facing installations.

AI chat is read-only by default. Users may grant read-and-edit access to one conversation only when they already have edit access to the current working design. AI changes remain a local proposal until a user explicitly applies them; the backend rechecks authorization, document validation, and revision at that point.

## Storage

When no `DATABASE_URL` is configured, Stratum uses an in-memory repository and displays a stateless-mode warning. Only administrators receive the shortcut to database settings. Test the database connection before applying it, and use a least-privileged runtime role in production. See [PostgreSQL operations](postgres-operations.md) for migration and rollout guidance.

## Access administration

Workspace permissions control reading, design creation, and management. Design permissions separately control reading, editing, commenting, reviewing, and management. Grants may target a user or an access group. The backend evaluates effective permissions for every request; UI visibility is only a usability aid.

## Okta OIDC Settings

For Okta SSO, create an OIDC Web Application in Okta and configure Stratum with:

- Okta domain, such as `https://company.okta.com`.
- Issuer. For the org authorization server this is the Okta domain. For a custom authorization server it is typically `https://company.okta.com/oauth2/{authorizationServerId}`.
- Client ID.
- Client secret.
- Sign-in redirect URI. This must exactly match the redirect URI registered in Okta.
- Sign-out redirect URI.
- Scopes. Start with `openid profile email`; include `groups` when group-to-role mapping is configured.
- Groups claim name, usually `groups`.
- Admin and reviewer group names for role mapping.
- Just-in-time provisioning, when users should be created from SSO claims on first sign-in.

Stratum uses the OIDC authorization-code flow with PKCE. It validates the provider signature, issuer, audience, expiry, nonce, one-time state, and verified email before creating a session. Configure multifactor and phishing-resistant authentication policies in Okta; Stratum inherits those controls rather than maintaining a second MFA system.

Access groups can map to values in the configured OIDC groups claim. On each SSO sign-in, mapped memberships are synchronized: newly asserted groups are added and stale mapped memberships are removed. Unmapped local groups are left unchanged.

For production deployments:

- Use HTTPS for the issuer, Stratum public URL, and redirect URI.
- Set `PUBLIC_URL` to the canonical Stratum browser origin.
- Register `/api/auth/oidc/callback` exactly as the sign-in redirect URI.
- Keep at least one sign-in method enabled while changing configuration.
- Verify discovery from the Admin Console before disabling local password sign-in.

References:

- Okta OIDC overview: https://developer.okta.com/docs/api/openapi/okta-oauth/guides/overview/
- Okta web app redirect sign-in guide: https://developer.okta.com/docs/guides/sign-into-web-app-redirect/main/
- Okta authorization servers: https://developer.okta.com/docs/concepts/auth-servers/
- Okta groups claim guide: https://developer.okta.com/docs/guides/customize-tokens-groups-claim/main/
