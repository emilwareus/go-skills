-- outbox/schema.sql defines the table that turns "publish to broker"
-- into "INSERT into a local table". Once the row is committed, the
-- outbox is the system of record for that message — the forwarder
-- guarantees delivery from there.
--
-- Relationship to the article: the Three Dots Labs post uses
-- Watermill's SQL Pub/Sub, which manages its own table layout (with
-- offsets/ack state). This schema is the hand-rolled equivalent so
-- you can see what's stored and why. The pattern is the same; only
-- the column names differ.
--
-- Why the outbox exists:
--
-- Without it, you have two systems (your DB and the broker) and no
-- atomic way to write to both. Whichever one you write to second can
-- fail, and now you either lost an event you committed, or you
-- published an event for a transaction that rolled back. Both are
-- bugs that look fine in dev and surface in prod.
--
-- With the outbox, the *only* write is to your own database, inside
-- the same transaction as the business state change. Either both land
-- or neither does. Publication becomes a separate, retryable concern.

CREATE TABLE outbox_messages (
    -- A UUID generated when the message is enqueued. This is the
    -- broker-level message ID and the basis for consumer-side dedup
    -- (see ../idempotent_consumer.go).
    id           UUID        PRIMARY KEY,

    -- Logical topic / event name. Keep this stable; it is part of
    -- your published contract.
    topic        TEXT        NOT NULL,

    -- Marshalled event payload (typically JSON). Store the bytes the
    -- broker will receive, not a Go struct — keep marshalling out of
    -- the forwarder so it is policy-free.
    payload      BYTEA       NOT NULL,

    -- Free-form headers/metadata: correlation IDs, tenant IDs, schema
    -- version, etc. JSONB so you can index on metadata.tenant_id if
    -- the forwarder needs per-tenant ordering.
    metadata     JSONB       NOT NULL DEFAULT '{}'::jsonb,

    -- Set by the producer transaction; never updated.
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Set by the forwarder after a successful publish. NULL means
    -- "still pending"; the forwarder query selects WHERE published_at
    -- IS NULL.
    published_at TIMESTAMPTZ,

    -- Crash-loop guard: count attempts so we can quarantine a
    -- poison message instead of looping forever.
    attempts     INT         NOT NULL DEFAULT 0,
    last_error   TEXT
);

-- The forwarder reads pending messages in insertion order. A partial
-- index keeps this cheap as the table grows: only unpublished rows
-- are indexed, so the hot scan stays small even with millions of
-- delivered messages still sitting in the table.
CREATE INDEX outbox_messages_pending_idx
    ON outbox_messages (created_at)
    WHERE published_at IS NULL;

-- Optional: an aggregate_id column + index if you need per-aggregate
-- ordering at the broker (Kafka partition key, Watermill ordering
-- key). Add it if you have an ordering requirement; do not add it
-- "just in case" — it makes the forwarder more complex.
