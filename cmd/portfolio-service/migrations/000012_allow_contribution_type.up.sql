-- Allow 'contribution' as an achievement type (2026-09-29).
--   Open-source work is an achievement the same way a certificate is: someone
--   else reviewed it, merged it and shipped it. The card, filters and evidence
--   button all read from this table, so it only needs the type opened up.
--   The check constraint is unnamed in 000009, so Postgres named it after the
--   column: achievements_type_check.

ALTER TABLE achievements.achievements
    DROP CONSTRAINT achievements_type_check;

ALTER TABLE achievements.achievements
    ADD CONSTRAINT achievements_type_check
    CHECK (type IN ('certificate', 'certification', 'webinar', 'seminar', 'contribution'));
