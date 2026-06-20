package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Session struct {
	ID               string
	UserID           string
	RefreshTokenHash []byte `json:"-"`
	TokenFamilyID    string
	ExpiresAt        time.Time
	RevokedAt        *time.Time
	UserAgent        *string
	CreatedAt        time.Time
}

type SessionState struct {
	UserID        string
	TokenFamilyID string
	ExpiresAt     time.Time
}

type SessionSummary struct {
	TokenFamilyID string
	UserAgent     string
	CreatedAt     time.Time
	ExpiresAt     time.Time
}

type SessionRotation struct {
	ID            string
	UserID        string
	TokenFamilyID string
	UserAgent     string
	CreatedAt     time.Time
	ExpiresAt     time.Time
}

type SessionStore struct {
	database *pgxpool.Pool
}

func NewSessionStore(
	database *pgxpool.Pool,
) *SessionStore {
	return &SessionStore{
		database: database,
	}
}

func (store *SessionStore) Create(
	ctx context.Context,
	session *Session,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if store == nil ||
		store.database == nil ||
		session == nil {
		return fmt.Errorf(
			"create session: %w",
			ErrDatabase,
		)
	}

	query := `
		INSERT INTO sessions (
			user_id,
			refresh_token_hash,
			expires_at,
			user_agent
		)
		VALUES (
			$1::uuid,
			$2,
			$3,
			$4
		)
		RETURNING
			id::text,
			token_family_id::text,
			created_at
	`

	queryContext, cancel := context.WithTimeout(
		ctx,
		queryTimeout,
	)
	defer cancel()

	err := store.database.QueryRow(
		queryContext,
		query,
		session.UserID,
		session.RefreshTokenHash,
		session.ExpiresAt,
		session.UserAgent,
	).Scan(
		&session.ID,
		&session.TokenFamilyID,
		&session.CreatedAt,
	)

	if err != nil {
		return mapCreateSessionError(err)
	}

	return nil
}

func (store *SessionStore) GetActiveState(
	ctx context.Context,
	userID string,
	tokenFamilyID string,
	now time.Time,
) (SessionState, error) {
	if err := ctx.Err(); err != nil {
		return SessionState{}, err
	}

	if store == nil ||
		store.database == nil {
		return SessionState{},
			fmt.Errorf(
				"get active session state: %w",
				ErrDatabase,
			)
	}

	query := `
		SELECT
			sessions.user_id::text,
			sessions.token_family_id::text,
			sessions.expires_at
		FROM sessions
		INNER JOIN users
			ON users.id = sessions.user_id
		WHERE sessions.user_id = $1::uuid
		  AND sessions.token_family_id = $2::uuid
		  AND sessions.revoked_at IS NULL
		  AND sessions.expires_at > $3
		  AND users.status = 'active'
		ORDER BY sessions.created_at DESC
		LIMIT 1
	`

	queryContext, cancel := context.WithTimeout(
		ctx,
		queryTimeout,
	)
	defer cancel()

	state := SessionState{}

	err := store.database.QueryRow(
		queryContext,
		query,
		userID,
		tokenFamilyID,
		now,
	).Scan(
		&state.UserID,
		&state.TokenFamilyID,
		&state.ExpiresAt,
	)
	if err != nil {
		return SessionState{},
			mapGetActiveSessionStateError(err)
	}

	return state, nil
}

func (store *SessionStore) ListActive(
	ctx context.Context,
	userID string,
	now time.Time,
) ([]SessionSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if store == nil ||
		store.database == nil {
		return nil, fmt.Errorf(
			"list active sessions: %w",
			ErrDatabase,
		)
	}

	query := `
		WITH active_families AS (
			SELECT DISTINCT ON (
				token_family_id
			)
				token_family_id,
				COALESCE(user_agent, '') AS user_agent,
				created_at,
				expires_at
			FROM sessions
			WHERE user_id = $1::uuid
			  AND revoked_at IS NULL
			  AND expires_at > $2
			ORDER BY
				token_family_id,
				created_at DESC
		)
		SELECT
			token_family_id::text,
			user_agent,
			created_at,
			expires_at
		FROM active_families
		ORDER BY created_at DESC
	`

	queryContext, cancel := context.WithTimeout(
		ctx,
		queryTimeout,
	)
	defer cancel()

	rows, err := store.database.Query(
		queryContext,
		query,
		userID,
		now,
	)
	if err != nil {
		return nil,
			mapListActiveSessionsError(err)
	}
	defer rows.Close()

	sessions := make([]SessionSummary, 0)

	for rows.Next() {
		summary := SessionSummary{}

		if err := rows.Scan(
			&summary.TokenFamilyID,
			&summary.UserAgent,
			&summary.CreatedAt,
			&summary.ExpiresAt,
		); err != nil {
			return nil,
				mapListActiveSessionsError(err)
		}

		sessions = append(
			sessions,
			summary,
		)
	}

	if err := rows.Err(); err != nil {
		return nil,
			mapListActiveSessionsError(err)
	}

	return sessions, nil
}

func (store *SessionStore) RevokeOwnedFamily(
	ctx context.Context,
	userID string,
	tokenFamilyID string,
	revokedAt time.Time,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if store == nil ||
		store.database == nil {
		return fmt.Errorf(
			"revoke session family: %w",
			ErrDatabase,
		)
	}

	query := `
		WITH revoked_sessions AS (
			UPDATE sessions
			SET revoked_at = $3
			WHERE user_id = $1::uuid
			  AND token_family_id = $2::uuid
			  AND revoked_at IS NULL
			RETURNING 1
		)
		SELECT count(*)
		FROM revoked_sessions
	`

	queryContext, cancel := context.WithTimeout(
		ctx,
		queryTimeout,
	)
	defer cancel()

	var revokedCount int

	err := store.database.QueryRow(
		queryContext,
		query,
		userID,
		tokenFamilyID,
		revokedAt,
	).Scan(&revokedCount)
	if err != nil {
		return mapRevokeSessionFamilyError(err)
	}

	if revokedCount == 0 {
		return ErrNotFound
	}

	return nil
}

func (store *SessionStore) RevokeAllForUser(
	ctx context.Context,
	userID string,
	revokedAt time.Time,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if store == nil ||
		store.database == nil {
		return fmt.Errorf(
			"revoke all user sessions: %w",
			ErrDatabase,
		)
	}

	query := `
		UPDATE sessions
		SET revoked_at = $2
		WHERE user_id = $1::uuid
		  AND revoked_at IS NULL
	`

	queryContext, cancel := context.WithTimeout(
		ctx,
		queryTimeout,
	)
	defer cancel()

	_, err := store.database.Exec(
		queryContext,
		query,
		userID,
		revokedAt,
	)
	if err != nil {
		return mapRevokeAllUserSessionsError(err)
	}

	return nil
}

func (store *SessionStore) RotateRefreshToken(
	ctx context.Context,
	currentRefreshTokenHash []byte,
	replacementRefreshTokenHash []byte,
	rotatedAt time.Time,
) (SessionRotation, error) {
	if err := ctx.Err(); err != nil {
		return SessionRotation{}, err
	}

	if store == nil ||
		store.database == nil {
		return SessionRotation{},
			fmt.Errorf(
				"rotate refresh token: %w",
				ErrDatabase,
			)
	}

	queryContext, cancel := context.WithTimeout(
		ctx,
		queryTimeout,
	)
	defer cancel()

	transaction, err := store.database.BeginTx(
		queryContext,
		pgx.TxOptions{},
	)
	if err != nil {
		return SessionRotation{},
			mapRotateRefreshTokenError(err)
	}

	committed := false

	defer func() {
		if !committed {
			_ = transaction.Rollback(
				queryContext,
			)
		}
	}()

	var (
		currentSessionID string
		userID           string
		tokenFamilyID    string
		expiresAt        time.Time
		alreadyRevoked   bool
		userAgent        string
		userStatus       string
	)

	err = transaction.QueryRow(
		queryContext,
		`
			SELECT
				sessions.id::text,
				sessions.user_id::text,
				sessions.token_family_id::text,
				sessions.expires_at,
				sessions.revoked_at IS NOT NULL,
				COALESCE(
					sessions.user_agent,
					''
				),
				users.status
			FROM sessions
			INNER JOIN users
				ON users.id = sessions.user_id
			WHERE sessions.refresh_token_hash = $1
			FOR UPDATE OF sessions
		`,
		currentRefreshTokenHash,
	).Scan(
		&currentSessionID,
		&userID,
		&tokenFamilyID,
		&expiresAt,
		&alreadyRevoked,
		&userAgent,
		&userStatus,
	)
	if err != nil {
		return SessionRotation{},
			mapRotateRefreshTokenError(err)
	}

	if alreadyRevoked {
		if err := revokeSessionFamilyInTransaction(
			queryContext,
			transaction,
			tokenFamilyID,
			rotatedAt,
		); err != nil {
			return SessionRotation{},
				mapRotateRefreshTokenError(err)
		}

		if err := transaction.Commit(
			queryContext,
		); err != nil {
			return SessionRotation{},
				mapRotateRefreshTokenError(err)
		}

		committed = true

		return SessionRotation{},
			ErrSessionReplayDetected
	}

	if !expiresAt.After(rotatedAt) {
		return SessionRotation{}, ErrNotFound
	}

	if userStatus != "active" {
		if err := revokeSessionFamilyInTransaction(
			queryContext,
			transaction,
			tokenFamilyID,
			rotatedAt,
		); err != nil {
			return SessionRotation{},
				mapRotateRefreshTokenError(err)
		}

		if err := transaction.Commit(
			queryContext,
		); err != nil {
			return SessionRotation{},
				mapRotateRefreshTokenError(err)
		}

		committed = true

		return SessionRotation{}, ErrNotFound
	}

	commandTag, err := transaction.Exec(
		queryContext,
		`
			UPDATE sessions
			SET revoked_at = $2
			WHERE id = $1::uuid
			  AND revoked_at IS NULL
		`,
		currentSessionID,
		rotatedAt,
	)
	if err != nil {
		return SessionRotation{},
			mapRotateRefreshTokenError(err)
	}

	if commandTag.RowsAffected() != 1 {
		return SessionRotation{},
			fmt.Errorf(
				"rotate refresh token: %w",
				ErrDatabase,
			)
	}

	rotation := SessionRotation{
		UserID:        userID,
		TokenFamilyID: tokenFamilyID,
		UserAgent:     userAgent,
		ExpiresAt:     expiresAt,
	}

	err = transaction.QueryRow(
		queryContext,
		`
			INSERT INTO sessions (
				user_id,
				refresh_token_hash,
				token_family_id,
				expires_at,
				user_agent
			)
			VALUES (
				$1::uuid,
				$2,
				$3::uuid,
				$4,
				NULLIF($5, '')
			)
			RETURNING
				id::text,
				created_at
		`,
		userID,
		replacementRefreshTokenHash,
		tokenFamilyID,
		expiresAt,
		userAgent,
	).Scan(
		&rotation.ID,
		&rotation.CreatedAt,
	)
	if err != nil {
		return SessionRotation{},
			mapRotateRefreshTokenError(err)
	}

	if err := transaction.Commit(
		queryContext,
	); err != nil {
		return SessionRotation{},
			mapRotateRefreshTokenError(err)
	}

	committed = true

	return rotation, nil
}

func revokeSessionFamilyInTransaction(
	ctx context.Context,
	transaction pgx.Tx,
	tokenFamilyID string,
	revokedAt time.Time,
) error {
	_, err := transaction.Exec(
		ctx,
		`
			UPDATE sessions
			SET revoked_at = $2
			WHERE token_family_id = $1::uuid
			  AND revoked_at IS NULL
		`,
		tokenFamilyID,
		revokedAt,
	)

	return err
}

func mapRotateRefreshTokenError(
	err error,
) error {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return ErrNotFound

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf(
			"rotate refresh token: %w",
			err,
		)

	default:
		return fmt.Errorf(
			"rotate refresh token: %w",
			ErrDatabase,
		)
	}
}

func mapCreateSessionError(err error) error {
	switch {
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf(
			"create session: %w",
			err,
		)

	default:
		return fmt.Errorf(
			"create session: %w",
			ErrDatabase,
		)
	}
}

func mapGetActiveSessionStateError(
	err error,
) error {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return ErrNotFound

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf(
			"get active session state: %w",
			err,
		)

	default:
		return fmt.Errorf(
			"get active session state: %w",
			ErrDatabase,
		)
	}
}

func mapListActiveSessionsError(
	err error,
) error {
	switch {
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf(
			"list active sessions: %w",
			err,
		)

	default:
		return fmt.Errorf(
			"list active sessions: %w",
			ErrDatabase,
		)
	}
}

func mapRevokeSessionFamilyError(
	err error,
) error {
	switch {
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf(
			"revoke session family: %w",
			err,
		)

	default:
		return fmt.Errorf(
			"revoke session family: %w",
			ErrDatabase,
		)
	}
}

func mapRevokeAllUserSessionsError(
	err error,
) error {
	switch {
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf(
			"revoke all user sessions: %w",
			err,
		)

	default:
		return fmt.Errorf(
			"revoke all user sessions: %w",
			ErrDatabase,
		)
	}
}
