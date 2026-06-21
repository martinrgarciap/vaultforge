DROP INDEX vault_items_active_by_vault_updated_at_idx;
DROP INDEX vault_items_deleted_by_vault_updated_at_idx;

CREATE INDEX vault_items_active_by_vault_updated_at_idx
    ON vault_items (
        vault_id,
        updated_at DESC,
        id DESC
    )
    WHERE deleted_at IS NULL;

CREATE INDEX vault_items_deleted_by_vault_updated_at_idx
    ON vault_items (
        vault_id,
        updated_at DESC,
        id DESC
    )
    WHERE deleted_at IS NOT NULL;

CREATE TABLE audit_outbox (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),

    event_type text NOT NULL,
    aggregate_type text NOT NULL,
    aggregate_id uuid NOT NULL,
    actor_id uuid NOT NULL,
    correlation_id text NOT NULL,

    sanitized_payload jsonb NOT NULL
        DEFAULT '{}'::jsonb,

    status text NOT NULL DEFAULT 'pending',
    attempts integer NOT NULL DEFAULT 0,

    created_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT audit_outbox_event_type_not_blank
        CHECK (btrim(event_type) <> ''),

    CONSTRAINT audit_outbox_aggregate_type_not_blank
        CHECK (btrim(aggregate_type) <> ''),

    CONSTRAINT audit_outbox_correlation_id_not_blank
        CHECK (btrim(correlation_id) <> ''),

    CONSTRAINT audit_outbox_sanitized_payload_object
        CHECK (
            jsonb_typeof(sanitized_payload) = 'object'
        ),

    CONSTRAINT audit_outbox_status_valid
        CHECK (
            status IN (
                'pending',
                'processing',
                'published',
                'failed'
            )
        ),

    CONSTRAINT audit_outbox_attempts_nonnegative
        CHECK (attempts >= 0)
);

CREATE INDEX audit_outbox_status_created_at_idx
    ON audit_outbox (
        status,
        created_at,
        id
    );

CREATE TABLE idempotency_records (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),

    actor_id uuid NOT NULL
        REFERENCES users(id)
        ON DELETE CASCADE,

    operation text NOT NULL,
    scope_id uuid NOT NULL,

    idempotency_key_hash bytea NOT NULL,
    request_hash bytea NOT NULL,

    resource_id uuid NOT NULL,

    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,

    CONSTRAINT idempotency_records_operation_not_blank
        CHECK (btrim(operation) <> ''),

    CONSTRAINT idempotency_records_key_hash_length
        CHECK (
            octet_length(idempotency_key_hash) = 32
        ),

    CONSTRAINT idempotency_records_request_hash_length
        CHECK (
            octet_length(request_hash) = 32
        ),

    CONSTRAINT idempotency_records_expires_after_creation
        CHECK (expires_at > created_at),

    CONSTRAINT idempotency_records_scope_key_unique
        UNIQUE (
            actor_id,
            operation,
            scope_id,
            idempotency_key_hash
        )
);

CREATE INDEX idempotency_records_expires_at_idx
    ON idempotency_records (expires_at);