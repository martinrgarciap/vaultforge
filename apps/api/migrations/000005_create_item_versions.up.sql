CREATE TABLE item_versions (
    vault_item_id uuid NOT NULL
        REFERENCES vault_items(id)
        ON DELETE CASCADE,

    version integer NOT NULL,

    item_type text NOT NULL,
    encrypted_payload bytea NOT NULL,
    nonce bytea NOT NULL,

    created_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT item_versions_primary_key
        PRIMARY KEY (vault_item_id, version),

    CONSTRAINT item_versions_version_positive
        CHECK (version > 0),

    CONSTRAINT item_versions_type_valid
        CHECK (
            item_type IN (
                'login',
                'api_key',
                'environment_variable',
                'database_connection',
                'secure_note'
            )
        ),

    CONSTRAINT item_versions_encrypted_payload_not_empty
        CHECK (octet_length(encrypted_payload) > 0),

    CONSTRAINT item_versions_nonce_not_empty
        CHECK (octet_length(nonce) > 0)
);