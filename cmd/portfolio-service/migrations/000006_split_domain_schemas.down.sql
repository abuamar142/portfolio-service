-- Restore the shared public registry (down of 000006).
-- Assumes links.tags ids were seeded from quotes.tags (guaranteed by up), so
-- merging back by id cannot collide on names.

ALTER TABLE links.link_tags DROP CONSTRAINT link_tags_tag_id_fkey;

ALTER TABLE quotes.quotes SET SCHEMA public;
ALTER TABLE quotes.quote_tags SET SCHEMA public;
ALTER TABLE quotes.tags SET SCHEMA public;
ALTER SEQUENCE quotes.tags_id_seq SET SCHEMA public;
ALTER TABLE public.tags ALTER COLUMN id SET DEFAULT nextval('tags_id_seq');

INSERT INTO public.tags (id, name)
    SELECT id, name FROM links.tags
    ON CONFLICT (id) DO NOTHING;
DROP TABLE links.tags;

ALTER TABLE links.link_tags
    ADD CONSTRAINT link_tags_tag_id_fkey FOREIGN KEY (tag_id)
    REFERENCES public.tags (id) ON DELETE CASCADE;
ALTER TABLE links.links SET SCHEMA public;
ALTER TABLE links.link_tags SET SCHEMA public;

DROP SCHEMA IF EXISTS quotes;
DROP SCHEMA IF EXISTS links;
