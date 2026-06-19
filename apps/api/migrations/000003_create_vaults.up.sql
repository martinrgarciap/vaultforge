CREATE TABLE vaults (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),

    owner_id uuid NOT NULL
        REFERENCES users(id)
        ON DELETE CASCADE,

    name text NOT NULL,

    crypto_version smallint,
    kdf_version smallint,
    salt bytea,
    wrapped_key bytea,

    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT vaults_name_not_blank
        CHECK (btrim(name) <> ''),

    CONSTRAINT vaults_crypto_version_positive
        CHECK (
            crypto_version IS NULL
            OR crypto_version > 0
        ),

    CONSTRAINT vaults_kdf_version_positive
        CHECK (
            kdf_version IS NULL
            OR kdf_version > 0
        ),

    CONSTRAINT vaults_salt_not_empty
        CHECK (
            salt IS NULL
            OR octet_length(salt) > 0
        ),

    CONSTRAINT vaults_wrapped_key_not_empty
        CHECK (
            wrapped_key IS NULL
            OR octet_length(wrapped_key) > 0
        ),

    CONSTRAINT vaults_crypto_metadata_complete
        CHECK (
            (
                crypto_version IS NULL
                AND kdf_version IS NULL
                AND salt IS NULL
                AND wrapped_key IS NULL
            )
            OR
            (
                crypto_version IS NOT NULL
                AND kdf_version IS NOT NULL
                AND salt IS NOT NULL
                AND wrapped_key IS NOT NULL
            )
        )
);

CREATE INDEX vaults_owner_id_updated_at_idx
    ON vaults (owner_id, updated_at DESC);