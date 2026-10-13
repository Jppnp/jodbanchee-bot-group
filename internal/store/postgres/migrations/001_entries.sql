CREATE TABLE entries (
    id         BIGSERIAL PRIMARY KEY,
    chat_id    TEXT        NOT NULL,
    user_id    TEXT        NOT NULL DEFAULT '',
    kind       TEXT        NOT NULL CHECK (kind IN ('income', 'expense')),
    amount     BIGINT      NOT NULL CHECK (amount > 0), -- satang
    note       TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL
);

-- Summaries read one chat over a time range.
CREATE INDEX entries_chat_created_idx ON entries (chat_id, created_at);

-- "ลบล่าสุด" finds a user's newest entry in a chat.
CREATE INDEX entries_chat_user_id_idx ON entries (chat_id, user_id, id DESC);
