-- Snippet Vault (2026-09-25).
--   snippets.*  — code snippets, each owning its own tag registry, mirroring
--   the links.* domain so tag vocabularies never mix across domains.

CREATE SCHEMA IF NOT EXISTS snippets;

CREATE TABLE IF NOT EXISTS snippets.snippets (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL,
    title       VARCHAR(255) NOT NULL,
    language    VARCHAR(50) NOT NULL,
    code        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_snippets_user ON snippets.snippets(user_id);
CREATE INDEX IF NOT EXISTS idx_snippets_created ON snippets.snippets(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_snippets_language ON snippets.snippets(language);

CREATE TABLE IF NOT EXISTS snippets.tags (
    id   serial PRIMARY KEY,
    name varchar(50) UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS snippets.snippet_tags (
    snippet_id UUID REFERENCES snippets.snippets(id) ON DELETE CASCADE,
    tag_id     INT  REFERENCES snippets.tags(id) ON DELETE CASCADE,
    PRIMARY KEY (snippet_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_snippet_tags_tag ON snippets.snippet_tags(tag_id);
