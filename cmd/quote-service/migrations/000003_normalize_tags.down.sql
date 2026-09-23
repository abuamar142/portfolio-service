-- Revert normalize tags: restore quote_tags with string tag column

-- 1. Add back tag column
ALTER TABLE quote_tags ADD COLUMN tag VARCHAR(50);

-- 2. Populate tag from tags table
UPDATE quote_tags qt SET tag = (
    SELECT name FROM tags WHERE id = qt.tag_id
);

-- 3. Set NOT NULL and drop constraints
ALTER TABLE quote_tags ALTER COLUMN tag SET NOT NULL;
ALTER TABLE quote_tags DROP CONSTRAINT quote_tags_pkey;
ALTER TABLE quote_tags DROP CONSTRAINT quote_tags_tag_id_fkey;
ALTER TABLE quote_tags DROP COLUMN tag_id;

-- 4. Add back index on tag
CREATE INDEX IF NOT EXISTS idx_quote_tags_tag ON quote_tags(tag);

-- 5. Drop tags table
DROP TABLE IF EXISTS tags;
