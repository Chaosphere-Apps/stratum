package store

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/system-design-evaluator/backend/internal/domain"
	"strings"
	"time"
)

func (r *PostgresRepository) ListUsers(ctx context.Context) ([]domain.User, error) {
	return listAllPages(ctx, func(ctx context.Context, options PageOptions) ([]domain.User, PageInfo, error) {
		return r.ListUsersPage(ctx, options)
	})
}

func (r *PostgresRepository) ListUsersPage(ctx context.Context, options PageOptions) ([]domain.User, PageInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, PageInfo{}, err
	}
	options = NormalizePageOptions(options)
	offset := OffsetFromCursor(options.Cursor)
	likeQuery := "%" + strings.ToLower(options.Query) + "%"
	rows, err := r.pool.Query(ctx, `
SELECT id, display_name, email, password_hash <> '', password_hash, role, status, last_seen_at, created_at, updated_at
FROM users
WHERE $1 = '' OR lower(display_name) LIKE $2 OR lower(email) LIKE $2 OR lower(role) LIKE $2
ORDER BY created_at ASC
LIMIT $3 OFFSET $4
`, options.Query, likeQuery, options.Limit+1, offset)
	if err != nil {
		return nil, PageInfo{}, err
	}
	defer rows.Close()

	users := []domain.User{}
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, PageInfo{}, err
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, PageInfo{}, err
	}
	items, page := pageFromFetched(users, options, offset)
	return items, page, nil
}

func (r *PostgresRepository) GetUser(ctx context.Context, userID string) (domain.User, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		userID = r.firstUserID(ctx)
	}
	row := r.pool.QueryRow(ctx, `
SELECT id, display_name, email, password_hash <> '', password_hash, role, status, last_seen_at, created_at, updated_at
FROM users
WHERE id = $1 AND status <> 'disabled'
`, userID)
	user, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, errors.New("user not found")
	}
	return user, err
}

func (r *PostgresRepository) AuthenticateUser(ctx context.Context, email string, password string) (domain.User, error) {
	now := r.clock().UTC()
	var failedAttempts int
	var lockedUntil *time.Time
	row := r.pool.QueryRow(ctx, `
SELECT id, display_name, email, password_hash <> '', password_hash, role, status, last_seen_at, created_at, updated_at,
       failed_login_attempts, locked_until
FROM users
WHERE lower(email) = lower($1) AND status <> 'disabled'
`, strings.TrimSpace(email))
	var user domain.User
	err := row.Scan(&user.ID, &user.DisplayName, &user.Email, &user.PasswordSet, &user.PasswordHash, &user.Role, &user.Status, &user.LastSeenAt, &user.CreatedAt, &user.UpdatedAt, &failedAttempts, &lockedUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, errors.New("invalid email or password")
	}
	if err != nil {
		return domain.User{}, err
	}
	if !user.PasswordSet || (lockedUntil != nil && lockedUntil.After(now)) {
		return domain.User{}, errors.New("invalid email or password")
	}
	if !verifyPassword(password, user.PasswordHash) {
		_, updateErr := r.pool.Exec(ctx, `
UPDATE users
SET failed_login_attempts = CASE WHEN failed_login_attempts + 1 >= 5 THEN 0 ELSE failed_login_attempts + 1 END,
    locked_until = CASE WHEN failed_login_attempts + 1 >= 5 THEN $1 + INTERVAL '15 minutes' ELSE locked_until END
WHERE id = $2
`, now, user.ID)
		if updateErr != nil {
			return domain.User{}, updateErr
		}
		return domain.User{}, errors.New("invalid email or password")
	}
	row = r.pool.QueryRow(ctx, `
UPDATE users
SET last_seen_at = $1, failed_login_attempts = 0, locked_until = NULL
WHERE id = $2
RETURNING id, display_name, email, password_hash <> '', password_hash, role, status, last_seen_at, created_at, updated_at
`, now, user.ID)
	return scanUser(row)
}

func (r *PostgresRepository) CreateSession(ctx context.Context, userID string, expiresAt time.Time) (string, error) {
	if _, err := r.GetUser(ctx, userID); err != nil {
		return "", err
	}
	token, tokenHash, err := newSessionToken()
	if err != nil {
		return "", err
	}
	now := r.clock().UTC()
	if expiresAt.IsZero() {
		expiresAt = now.Add(12 * time.Hour)
	}
	if _, err := r.pool.Exec(ctx, `
INSERT INTO auth_sessions (token_hash, user_id, created_at, last_seen_at, expires_at)
VALUES ($1, $2, $3, $3, $4)
`, tokenHash, userID, now, expiresAt.UTC()); err != nil {
		return "", err
	}
	return token, nil
}

func (r *PostgresRepository) GetUserBySessionToken(ctx context.Context, token string) (domain.User, error) {
	var userID string
	if err := r.pool.QueryRow(ctx, `
UPDATE auth_sessions
SET last_seen_at = $1
WHERE token_hash = $2 AND expires_at > $1
RETURNING user_id
`, r.clock().UTC(), sessionTokenHash(token)).Scan(&userID); errors.Is(err, pgx.ErrNoRows) {
		_, _ = r.pool.Exec(ctx, `DELETE FROM auth_sessions WHERE token_hash = $1`, sessionTokenHash(token))
		return domain.User{}, errors.New("session not found")
	} else if err != nil {
		return domain.User{}, err
	}
	return r.GetUser(ctx, userID)
}

func (r *PostgresRepository) DeleteSession(ctx context.Context, token string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM auth_sessions WHERE token_hash = $1`, sessionTokenHash(token))
	return err
}

func (r *PostgresRepository) PasswordSetupRequired(ctx context.Context) (bool, error) {
	var userCount int
	var passwordCount int
	if err := r.pool.QueryRow(ctx, `
SELECT COUNT(*), COUNT(*) FILTER (WHERE password_hash <> '')
FROM users
`).Scan(&userCount, &passwordCount); err != nil {
		return false, err
	}
	return userCount > 0 && passwordCount == 0, nil
}

func (r *PostgresRepository) SetInitialAdminPassword(ctx context.Context, email string, password string) (domain.User, error) {
	requiresSetup, err := r.PasswordSetupRequired(ctx)
	if err != nil {
		return domain.User{}, err
	}
	if !requiresSetup {
		return domain.User{}, errors.New("password setup is not available")
	}
	passwordHash, err := hashPassword(password)
	if err != nil {
		return domain.User{}, err
	}
	row := r.pool.QueryRow(ctx, `
UPDATE users
SET password_hash = $1, updated_at = $2
WHERE lower(email) = lower($3) AND role = 'admin' AND status <> 'disabled'
RETURNING id, display_name, email, password_hash <> '', password_hash, role, status, last_seen_at, created_at, updated_at
`, passwordHash, r.clock().UTC(), strings.TrimSpace(email))
	user, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, errors.New("admin user not found")
	}
	return user, err
}

func (r *PostgresRepository) CreateFirstAdmin(ctx context.Context, displayName string, email string, password string) (domain.User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.User{}, err
	}
	defer rollback(ctx, tx)

	var count int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return domain.User{}, err
	}
	if count > 0 {
		return domain.User{}, errors.New("organization already has users")
	}
	user, err := createUserWithTx(ctx, tx, r.clock().UTC(), displayName, email, "admin", password)
	if err != nil {
		if isUniqueViolation(err, "users_email_key") {
			return domain.User{}, errors.New("a user with this email already exists")
		}
		return domain.User{}, err
	}
	if err := claimLegacyGuestData(ctx, tx, user.ID); err != nil {
		return domain.User{}, err
	}
	if err := ensureDefaultSignInConfig(ctx, tx, r.clock().UTC()); err != nil {
		return domain.User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (r *PostgresRepository) CreateUser(ctx context.Context, displayName string, email string, role string, password string) (domain.User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.User{}, err
	}
	defer rollback(ctx, tx)

	var count int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return domain.User{}, err
	}
	if count == 0 {
		return domain.User{}, errors.New("create the first admin before inviting users")
	}
	user, err := createUserWithTx(ctx, tx, r.clock().UTC(), displayName, email, role, password)
	if err != nil {
		if isUniqueViolation(err, "users_email_key") {
			return domain.User{}, errors.New("a user with this email already exists")
		}
		return domain.User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (r *PostgresRepository) ImportUsers(ctx context.Context, users []domain.User) (int, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer rollback(ctx, tx)

	var count int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return 0, err
	}
	if count > 0 {
		return 0, errors.New("database already has users")
	}
	now := r.clock().UTC()
	for _, user := range users {
		user.ID = strings.TrimSpace(user.ID)
		user.DisplayName = strings.TrimSpace(user.DisplayName)
		user.Email = strings.ToLower(strings.TrimSpace(user.Email))
		user.Role = normalizedUserRole(user.Role)
		user.Status = normalizedUserStatus(user.Status)
		if user.ID == "" || user.DisplayName == "" || user.Email == "" {
			return 0, errors.New("all migrated users require id, display name, and email")
		}
		if user.CreatedAt.IsZero() {
			user.CreatedAt = now
		}
		if user.UpdatedAt.IsZero() {
			user.UpdatedAt = now
		}
		if user.LastSeenAt.IsZero() {
			user.LastSeenAt = user.UpdatedAt
		}
		if !user.PasswordSet {
			user.PasswordHash = ""
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO users (id, display_name, email, password_hash, role, status, last_seen_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
`, user.ID, user.DisplayName, user.Email, user.PasswordHash, user.Role, user.Status, user.LastSeenAt, user.CreatedAt, user.UpdatedAt); err != nil {
			if isUniqueViolation(err, "users_email_key") {
				return 0, errors.New("a user with this email already exists")
			}
			return 0, err
		}
	}
	if err := ensureDefaultSignInConfig(ctx, tx, now); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(users), nil
}

func (r *PostgresRepository) UpdateUser(ctx context.Context, userID string, displayName string, email string, role string, status string, password string) (domain.User, error) {
	existing, err := r.GetUser(ctx, userID)
	if err != nil {
		return domain.User{}, err
	}
	if strings.TrimSpace(displayName) != "" {
		existing.DisplayName = strings.TrimSpace(displayName)
	}
	if strings.TrimSpace(email) != "" {
		existing.Email = strings.ToLower(strings.TrimSpace(email))
	}
	if strings.TrimSpace(role) != "" {
		existing.Role = normalizedUserRole(role)
	}
	if strings.TrimSpace(status) != "" {
		existing.Status = normalizedUserStatus(status)
	}
	if strings.TrimSpace(password) != "" {
		passwordHash, err := hashPassword(password)
		if err != nil {
			return domain.User{}, err
		}
		existing.PasswordHash = passwordHash
		existing.PasswordSet = true
	}
	existing.UpdatedAt = r.clock().UTC()
	row := r.pool.QueryRow(ctx, `
UPDATE users
SET display_name = $1, email = $2, password_hash = $3, role = $4, status = $5, updated_at = $6
WHERE id = $7
RETURNING id, display_name, email, password_hash <> '', password_hash, role, status, last_seen_at, created_at, updated_at
`, existing.DisplayName, existing.Email, existing.PasswordHash, existing.Role, existing.Status, existing.UpdatedAt, existing.ID)
	user, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, errors.New("user not found")
	}
	if isUniqueViolation(err, "users_email_key") {
		return domain.User{}, errors.New("a user with this email already exists")
	}
	return user, err
}

func (r *PostgresRepository) CreatePasswordResetToken(ctx context.Context, userID string, expiresAt time.Time) (string, domain.PasswordResetToken, error) {
	user, err := r.GetUser(ctx, userID)
	if err != nil {
		return "", domain.PasswordResetToken{}, err
	}
	if user.Status == "disabled" {
		return "", domain.PasswordResetToken{}, errors.New("user is disabled")
	}
	token, tokenHash, err := newPasswordResetToken()
	if err != nil {
		return "", domain.PasswordResetToken{}, err
	}
	now := r.clock().UTC()
	if expiresAt.IsZero() {
		expiresAt = now.Add(time.Hour)
	}
	reset := domain.PasswordResetToken{
		Token:     tokenHash,
		UserID:    user.ID,
		CreatedAt: now,
		ExpiresAt: expiresAt.UTC(),
	}
	if _, err := r.pool.Exec(ctx, `
INSERT INTO password_reset_tokens (token_hash, user_id, created_at, expires_at)
VALUES ($1, $2, $3, $4)
`, tokenHash, user.ID, reset.CreatedAt, reset.ExpiresAt); err != nil {
		return "", domain.PasswordResetToken{}, err
	}
	return token, reset, nil
}

func (r *PostgresRepository) ResetPasswordWithToken(ctx context.Context, token string, password string) (domain.User, error) {
	passwordHash, err := hashPassword(password)
	if err != nil {
		return domain.User{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.User{}, err
	}
	defer rollback(ctx, tx)

	now := r.clock().UTC()
	var userID string
	if err := tx.QueryRow(ctx, `
UPDATE password_reset_tokens
SET used_at = $1
WHERE token_hash = $2 AND used_at IS NULL AND expires_at > $1
RETURNING user_id
`, now, sessionTokenHash(token)).Scan(&userID); errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, errors.New("password reset link is invalid or expired")
	} else if err != nil {
		return domain.User{}, err
	}
	row := tx.QueryRow(ctx, `
UPDATE users
SET password_hash = $1, updated_at = $2
WHERE id = $3 AND status <> 'disabled'
RETURNING id, display_name, email, password_hash <> '', password_hash, role, status, last_seen_at, created_at, updated_at
`, passwordHash, now, userID)
	user, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, errors.New("password reset link is invalid or expired")
	}
	if err != nil {
		return domain.User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (r *PostgresRepository) DeleteUser(ctx context.Context, userID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)

	var role string
	if err := tx.QueryRow(ctx, `SELECT role FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&role); errors.Is(err, pgx.ErrNoRows) {
		return errors.New("user not found")
	} else if err != nil {
		return err
	}
	if role == "admin" {
		var adminCount int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE role = 'admin' AND status <> 'disabled'`).Scan(&adminCount); err != nil {
			return err
		}
		if adminCount <= 1 {
			return errors.New("at least one admin is required")
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) GetSignInConfig(ctx context.Context) (domain.SignInConfig, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.SignInConfig{}, err
	}
	defer rollback(ctx, tx)
	if err := ensureDefaultSignInConfig(ctx, tx, r.clock().UTC()); err != nil {
		return domain.SignInConfig{}, err
	}
	config, err := getSignInConfigWithTx(ctx, tx)
	if err != nil {
		return domain.SignInConfig{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.SignInConfig{}, err
	}
	return config, nil
}

func (r *PostgresRepository) GetSignInConfigWithSecret(ctx context.Context) (domain.SignInConfig, error) {
	if _, err := r.GetSignInConfig(ctx); err != nil {
		return domain.SignInConfig{}, err
	}
	row := r.pool.QueryRow(ctx, `
SELECT local_password_enabled, sso_enabled, provider, okta_domain, issuer, client_id, client_secret,
	redirect_uri, post_logout_redirect_uri, scopes, groups_claim, admin_group, reviewer_group, jit_provisioning, updated_at
FROM sign_in_settings
WHERE id = 'default'
`)
	return scanSignInConfigWithSecret(row)
}

func (r *PostgresRepository) UpdateSignInConfig(ctx context.Context, config domain.SignInConfig, clientSecret string) (domain.SignInConfig, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.SignInConfig{}, err
	}
	defer rollback(ctx, tx)
	if err := ensureDefaultSignInConfig(ctx, tx, r.clock().UTC()); err != nil {
		return domain.SignInConfig{}, err
	}
	currentSecret := ""
	if err := tx.QueryRow(ctx, `SELECT client_secret FROM sign_in_settings WHERE id = 'default' FOR UPDATE`).Scan(&currentSecret); err != nil {
		return domain.SignInConfig{}, err
	}
	if strings.TrimSpace(clientSecret) != "" {
		currentSecret = strings.TrimSpace(clientSecret)
	}
	updatedAt := r.clock().UTC()
	row := tx.QueryRow(ctx, `
UPDATE sign_in_settings
SET local_password_enabled = $1,
	sso_enabled = $2,
	provider = $3,
	okta_domain = $4,
	issuer = $5,
	client_id = $6,
	client_secret = $7,
	redirect_uri = $8,
	post_logout_redirect_uri = $9,
	scopes = $10,
	groups_claim = $11,
	admin_group = $12,
	reviewer_group = $13,
	jit_provisioning = $14,
	updated_at = $15
WHERE id = 'default'
RETURNING local_password_enabled, sso_enabled, provider, okta_domain, issuer, client_id, client_secret <> '', redirect_uri, post_logout_redirect_uri, scopes, groups_claim, admin_group, reviewer_group, jit_provisioning, updated_at
`, config.LocalPasswordEnabled, config.SSOEnabled, strings.TrimSpace(config.Provider), strings.TrimSpace(config.OktaDomain), strings.TrimSpace(config.Issuer), strings.TrimSpace(config.ClientID), currentSecret, strings.TrimSpace(config.RedirectURI), strings.TrimSpace(config.PostLogoutRedirectURI), normalizedScopes(config.Scopes), strings.TrimSpace(config.GroupsClaim), strings.TrimSpace(config.AdminGroup), strings.TrimSpace(config.ReviewerGroup), config.JITProvisioning, updatedAt)
	next, err := scanSignInConfig(row)
	if err != nil {
		return domain.SignInConfig{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.SignInConfig{}, err
	}
	return next, nil
}

func (r *PostgresRepository) CreateOIDCFlow(ctx context.Context, flow domain.OIDCFlow) error {
	if strings.TrimSpace(flow.StateHash) == "" || strings.TrimSpace(flow.Nonce) == "" || strings.TrimSpace(flow.CodeVerifier) == "" {
		return errors.New("OIDC flow is incomplete")
	}
	now := r.clock().UTC()
	if flow.ExpiresAt.IsZero() {
		flow.ExpiresAt = now.Add(10 * time.Minute)
	}
	_, err := r.pool.Exec(ctx, `
INSERT INTO oidc_flows (state_hash, nonce, code_verifier, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (state_hash) DO NOTHING
`, flow.StateHash, flow.Nonce, flow.CodeVerifier, flow.ExpiresAt.UTC(), now)
	return err
}

func (r *PostgresRepository) ConsumeOIDCFlow(ctx context.Context, state string) (domain.OIDCFlow, error) {
	var flow domain.OIDCFlow
	stateHash := sessionTokenHash(state)
	err := r.pool.QueryRow(ctx, `
DELETE FROM oidc_flows
WHERE state_hash = $1 AND expires_at > $2
RETURNING state_hash, nonce, code_verifier, expires_at
`, stateHash, r.clock().UTC()).Scan(&flow.StateHash, &flow.Nonce, &flow.CodeVerifier, &flow.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.OIDCFlow{}, errors.New("OIDC sign-in request is invalid or expired")
	}
	return flow, err
}

func (r *PostgresRepository) ResolveOIDCUser(ctx context.Context, identity domain.OIDCIdentity, jitProvisioning bool) (domain.User, error) {
	provider := strings.ToLower(strings.TrimSpace(identity.Provider))
	subject := strings.TrimSpace(identity.Subject)
	email := strings.ToLower(strings.TrimSpace(identity.Email))
	if provider == "" || subject == "" || email == "" {
		return domain.User{}, errors.New("OIDC identity is incomplete")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.User{}, err
	}
	defer rollback(ctx, tx)

	row := tx.QueryRow(ctx, `
SELECT users.id, users.display_name, users.email, users.password_hash <> '', users.password_hash,
	users.role, users.status, users.last_seen_at, users.created_at, users.updated_at
FROM user_identities AS identity
JOIN users ON users.id = identity.user_id
WHERE identity.provider = $1 AND identity.subject = $2 AND users.status <> 'disabled'
`, provider, subject)
	user, scanErr := scanUser(row)
	now := r.clock().UTC()
	if scanErr == nil {
		role := normalizedUserRole(identity.Role)
		if user.Role == "admin" {
			role = "admin"
		}
		displayName := strings.TrimSpace(identity.DisplayName)
		if displayName == "" {
			displayName = user.DisplayName
		}
		row = tx.QueryRow(ctx, `
UPDATE users SET display_name = $1, role = $2, last_seen_at = $3, updated_at = $3
WHERE id = $4
RETURNING id, display_name, email, password_hash <> '', password_hash, role, status, last_seen_at, created_at, updated_at
`, displayName, role, now, user.ID)
		user, err = scanUser(row)
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE user_identities SET updated_at = $1 WHERE provider = $2 AND subject = $3`, now, provider, subject)
		}
	} else if !errors.Is(scanErr, pgx.ErrNoRows) {
		return domain.User{}, scanErr
	} else {
		row = tx.QueryRow(ctx, `
SELECT id, display_name, email, password_hash <> '', password_hash, role, status, last_seen_at, created_at, updated_at
FROM users WHERE lower(email) = lower($1) AND status <> 'disabled'
`, email)
		user, scanErr = scanUser(row)
		if errors.Is(scanErr, pgx.ErrNoRows) {
			if !jitProvisioning {
				return domain.User{}, errors.New("user is not provisioned for Stratum")
			}
			user, err = createUserWithTx(ctx, tx, now, identity.DisplayName, email, identity.Role, "")
		} else {
			err = scanErr
		}
		if err == nil {
			_, err = tx.Exec(ctx, `
INSERT INTO user_identities (provider, subject, user_id, created_at, updated_at)
VALUES ($1, $2, $3, $4, $4)
ON CONFLICT (provider, subject) DO UPDATE SET user_id = EXCLUDED.user_id, updated_at = EXCLUDED.updated_at
`, provider, subject, user.ID, now)
		}
	}
	if err != nil {
		return domain.User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (r *PostgresRepository) SyncUserAccessGroups(ctx context.Context, userID string, externalGroupNames []string) error {
	claims := make([]string, 0, len(externalGroupNames))
	for _, name := range externalGroupNames {
		if normalized := strings.ToLower(strings.TrimSpace(name)); normalized != "" {
			claims = append(claims, normalized)
		}
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)
	if _, err := tx.Exec(ctx, `
DELETE FROM access_group_members AS member
USING access_groups AS access_group
WHERE member.group_id = access_group.id AND member.user_id = $1
	AND access_group.okta_group_name <> ''
	AND NOT (lower(access_group.okta_group_name) = ANY($2::text[]))
`, userID, claims); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO access_group_members (group_id, user_id, added_at)
SELECT id, $1, $3 FROM access_groups
WHERE okta_group_name <> '' AND lower(okta_group_name) = ANY($2::text[])
ON CONFLICT (group_id, user_id) DO NOTHING
`, userID, claims, r.clock().UTC()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
