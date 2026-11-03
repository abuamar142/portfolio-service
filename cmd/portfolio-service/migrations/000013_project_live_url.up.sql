-- Project live URL (2026-11-03).
--   profile.projects only ever had github_url, so the API could not tell a
--   visitor where to actually open a project — every card linked to source and
--   nothing else. The frontend type already declared liveUrl; it was always
--   empty because nothing filled it.
--
--   Empty string means "no live deployment" (matches the github_url
--   convention), so the API keeps returning a string and the UI decides
--   whether to render a demo link.
--
--   Backfill is deliberately conservative: only URLs verified to answer 200 at
--   migration time, and only rows matched by their exact title. Anything that
--   later dies is a data edit, not a migration.

ALTER TABLE profile.projects
    ADD COLUMN IF NOT EXISTS live_url VARCHAR(400) NOT NULL DEFAULT '';

UPDATE profile.projects
SET live_url = v.url
FROM (VALUES
    ('Hafalan - Quran Tracker', 'https://hafalan-delta.vercel.app'),
    ('Pondok Pesantren Asy-Syaikhoni', 'https://asyaikhoni.abuamar.online')
) AS v(title, url)
WHERE profile.projects.title = v.title
  AND profile.projects.live_url = '';
