-- Split domains into per-domain schemas (2026-09-25).
--   public.*  ->  quotes.* and links.*
-- Each domain owns its tables AND its own tags registry, so tag vocabularies
-- can never mix across domains.

CREATE SCHEMA IF NOT EXISTS quotes;
CREATE SCHEMA IF NOT EXISTS links;

-- Quote domain: indexes + FK constraints follow their tables automatically.
-- Move the owning TABLE first: an owned sequence may only be moved into the
-- schema of its owning table.
ALTER TABLE public.quotes SET SCHEMA quotes;
ALTER TABLE public.quote_tags SET SCHEMA quotes;
ALTER TABLE public.tags SET SCHEMA quotes;
-- The owned sequence tags_id_seq follows its table automatically; the serial
-- default was stored as an unqualified name, so rewrite it qualified.
ALTER TABLE quotes.tags ALTER COLUMN id SET DEFAULT nextval('quotes.tags_id_seq');

-- Link domain.
ALTER TABLE public.links SET SCHEMA links;
ALTER TABLE public.link_tags SET SCHEMA links;

-- Links-local tags registry (ids preserved so link_tags.tag_id stays valid).
CREATE TABLE links.tags (
    id   integer PRIMARY KEY,
    name varchar(50) UNIQUE NOT NULL
);
INSERT INTO links.tags (id, name)
    SELECT DISTINCT t.id, t.name
    FROM quotes.tags t
    JOIN links.link_tags lt ON lt.tag_id = t.id
    ON CONFLICT (id) DO NOTHING;
CREATE SEQUENCE links.tags_id_seq;
ALTER TABLE links.tags ALTER COLUMN id SET DEFAULT nextval('links.tags_id_seq');
SELECT setval('links.tags_id_seq', (SELECT GREATEST(COALESCE(MAX(id), 0), 1) FROM links.tags));
ALTER SEQUENCE links.tags_id_seq OWNED BY links.tags.id;

-- link_tags.tag_id still referenced the original registry (now quotes.tags);
-- repoint it to the links-local registry.
ALTER TABLE links.link_tags DROP CONSTRAINT link_tags_tag_id_fkey;
ALTER TABLE links.link_tags
    ADD CONSTRAINT link_tags_tag_id_fkey FOREIGN KEY (tag_id)
    REFERENCES links.tags (id) ON DELETE CASCADE;
