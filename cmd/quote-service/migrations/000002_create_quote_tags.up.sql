CREATE TABLE IF NOT EXISTS quote_tags (
    id       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    quote_id UUID REFERENCES quotes(id) ON DELETE CASCADE,
    tag      VARCHAR(50) NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_quote_tags_quote ON quote_tags(quote_id);
CREATE INDEX IF NOT EXISTS idx_quote_tags_tag ON quote_tags(tag);
