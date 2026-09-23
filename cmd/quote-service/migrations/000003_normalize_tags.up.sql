-- Normalize tags: create tags table and update quote_tags

-- 1. Create tags table
CREATE TABLE IF NOT EXISTS tags (
    id   SERIAL PRIMARY KEY,
    name VARCHAR(50) UNIQUE NOT NULL
);

-- 2. Insert unique tags from quote_tags
INSERT INTO tags (name) SELECT DISTINCT tag FROM quote_tags;

-- 3. Add tag_id column to quote_tags
ALTER TABLE quote_tags ADD COLUMN tag_id INT;

-- 4. Populate tag_id based on name
UPDATE quote_tags qt SET tag_id = (
    SELECT id FROM tags WHERE name = qt.tag
);

-- 5. Drop old tag column, set NOT NULL
ALTER TABLE quote_tags DROP COLUMN tag;
ALTER TABLE quote_tags ALTER COLUMN tag_id SET NOT NULL;

-- 6. Add composite primary key and foreign key
ALTER TABLE quote_tags ADD PRIMARY KEY (quote_id, tag_id);
ALTER TABLE quote_tags ADD FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE;
