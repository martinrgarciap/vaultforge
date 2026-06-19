CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),

    email text NOT NULL,
    password_hash text NOT NULL,
    password_algorithm text NOT NULL,

    status text NOT NULL DEFAULT 'active',

    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT users_email_not_blank
        CHECK (btrim(email) <> ''),

    CONSTRAINT users_email_normalized
        CHECK (email = lower(btrim(email))),

    CONSTRAINT users_password_hash_not_blank
        CHECK (btrim(password_hash) <> ''),

    CONSTRAINT users_password_algorithm_not_blank
        CHECK (btrim(password_algorithm) <> ''),

    CONSTRAINT users_status_valid
        CHECK (status IN ('active', 'disabled')),

    CONSTRAINT users_email_unique
        UNIQUE (email)
);