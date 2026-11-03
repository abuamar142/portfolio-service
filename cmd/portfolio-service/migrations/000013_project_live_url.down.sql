-- Reverse: drop the live URL column.
--   Nothing else depends on live_url; the API simply stops returning it and the
--   UI falls back to repo-only links.

ALTER TABLE profile.projects
    DROP COLUMN IF EXISTS live_url;
