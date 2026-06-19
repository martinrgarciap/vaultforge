CREATE TABLE sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),

    user_id uuid NOT NULL
        REFERENCES users(id)
        ON DELETE CASCADE,

    refresh_token_hash bytea NOT NULL,
    token_family_id uuid NOT NULL DEFAULT gen_random_uuid(),

    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,

    user_agent text,
    created_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT sessions_refresh_token_hash_not_empty
        CHECK (octet_length(refresh_token_hash) > 0),

    CONSTRAINT sessions_expires_after_creation
        CHECK (expires_at > created_at),

    CONSTRAINT sessions_revoked_after_creation
        CHECK (
            revoked_at IS NULL
            OR revoked_at >= created_at
        ),

    CONSTRAINT sessions_user_agent_not_blank
        CHECK (
            user_agent IS NULL
            OR btrim(user_agent) <> ''
        ),

    CONSTRAINT sessions_refresh_token_hash_unique
        UNIQUE (refresh_token_hash)
);

CREATE INDEX sessions_user_id_idx
    ON sessions (user_id);

CREATE INDEX sessions_token_family_id_idx
    ON sessions (token_family_id);

CREATE INDEX sessions_expires_at_idx
    ON sessions (expires_at);