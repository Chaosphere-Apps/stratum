# Administration and SSO

Stratum starts in onboarding mode when no users exist. The first user created from the onboarding screen becomes the organization admin.

Admins can open the Admin Console from the top navigation to manage:

- Users and product roles: admin, architect, reviewer, and member.
- User status: active or disabled.
- Sign-in methods: local password sign-in and SSO.
- Okta OpenID Connect settings.

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
