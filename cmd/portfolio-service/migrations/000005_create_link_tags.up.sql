CREATE TABLE IF NOT EXISTS link_tags (
    link_id UUID REFERENCES links(id) ON DELETE CASCADE,
    tag_id  INT   REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (link_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_link_tags_tag ON link_tags(tag_id);
