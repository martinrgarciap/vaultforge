DROP TABLE IF EXISTS idempotency_records;
DROP TABLE IF EXISTS audit_outbox;

DROP INDEX IF EXISTS
    vault_items_active_by_vault_updated_at_idx;

DROP INDEX IF EXISTS
    vault_items_deleted_by_vault_updated_at_idx;

CREATE INDEX vault_items_active_by_vault_updated_at_idx
    ON vault_items (vault_id, updated_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX vault_items_deleted_by_vault_updated_at_idx
    ON vault_items (vault_id, updated_at DESC)
    WHERE deleted_at IS NOT NULL;