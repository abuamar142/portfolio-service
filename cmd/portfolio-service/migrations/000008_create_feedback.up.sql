-- Feedback inbox (2026-09-26).
--   feedback.* — short anonymous visitor messages for the owner dashboard,
--   laid out like the other per-domain schemas (quotes.*, links.*, snippets.*).

CREATE SCHEMA IF NOT EXISTS feedback;

CREATE TABLE IF NOT EXISTS feedback.feedback (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message    VARCHAR(500) NOT NULL,
    contact    VARCHAR(120) NOT NULL DEFAULT '',
    page_url   VARCHAR(300) NOT NULL DEFAULT '',
    status     VARCHAR(10)  NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'read')),
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_feedback_status ON feedback.feedback (status);
CREATE INDEX IF NOT EXISTS idx_feedback_created ON feedback.feedback (created_at DESC);
