CREATE TABLE vault_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),

    vault_id uuid NOT NULL
        REFERENCES vaults(id)
        ON DELETE CASCADE,

    item_type text NOT NULL,

    encrypted_payload bytea NOT NULL,
    nonce bytea NOT NULL,

    version integer NOT NULL DEFAULT 1,

    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,

    CONSTRAINT vault_items_type_valid
        CHECK (
            item_type IN (
                'login',
                'api_key',
                'environment_variable',
                'database_connection',
                'secure_note'
            )
        ),

    CONSTRAINT vault_items_encrypted_payload_not_empty
        CHECK (octet_length(encrypted_payload) > 0),

    CONSTRAINT vault_items_nonce_not_empty
        CHECK (octet_length(nonce) > 0),

    CONSTRAINT vault_items_version_positive
        CHECK (version > 0),

    CONSTRAINT vault_items_updated_after_creation
        CHECK (updated_at >= created_at),

    CONSTRAINT vault_items_deleted_after_creation
        CHECK (
            deleted_at IS NULL
            OR deleted_at >= created_at
        )
);

CREATE INDEX vault_items_active_by_vault_updated_at_idx
    ON vault_items (vault_id, updated_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX vault_items_deleted_by_vault_updated_at_idx
    ON vault_items (vault_id, updated_at DESC)
    WHERE deleted_at IS NOT NULL;