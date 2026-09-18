CREATE TABLE IF NOT EXISTS quotes (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL,
    content      TEXT NOT NULL CHECK (length(content) <= 500),
    author_name  VARCHAR(100) DEFAULT '',
    is_anonymous BOOLEAN DEFAULT false,
    source       VARCHAR(255) DEFAULT '',
    color        VARCHAR(20) DEFAULT 'white',
    created_at   TIMESTAMPTZ DEFAULT NOW(),
    updated_at   TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_quotes_user ON quotes(user_id);
CREATE INDEX IF NOT EXISTS idx_quotes_created ON quotes(created_at DESC);
