-- Revert normalize tags: restore quote_tags with string tag column

-- 1. Drop constraints
ALTER TABLE quote_tags DROP CONSTRAINT quote_tags_pkey;
ALTER TABLE quote_tags DROP CONSTRAINT quote_tags_tag_id_fkey;

-- 2. Add back id and tag columns
ALTER TABLE quote_tags ADD COLUMN id UUID PRIMARY KEY DEFAULT gen_random_uuid();
ALTER TABLE quote_tags ADD COLUMN tag VARCHAR(50);

-- 3. Populate tag from tags table
UPDATE quote_tags qt SET tag = (
    SELECT name FROM tags WHERE id = qt.tag_id
);

-- 4. Set NOT NULL and drop tag_id
ALTER TABLE quote_tags ALTER COLUMN tag SET NOT NULL;
ALTER TABLE quote_tags DROP COLUMN tag_id;

-- 5. Add back indexes
CREATE INDEX IF NOT EXISTS idx_quote_tags_quote ON quote_tags(quote_id);
CREATE INDEX IF NOT EXISTS idx_quote_tags_tag ON quote_tags(tag);

-- 6. Add back FK to quotes
ALTER TABLE quote_tags ADD FOREIGN KEY (quote_id) REFERENCES quotes(id) ON DELETE CASCADE;

-- 7. Drop tags table
DROP TABLE IF EXISTS tags;
