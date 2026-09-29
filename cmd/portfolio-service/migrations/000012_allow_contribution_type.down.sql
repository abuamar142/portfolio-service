-- Reverse: disallow 'contribution' as an achievement type.
-- Rows already using it must be moved to another type or removed first,
-- otherwise the narrower constraint cannot be applied.

ALTER TABLE achievements.achievements
    DROP CONSTRAINT achievements_type_check;

ALTER TABLE achievements.achievements
    ADD CONSTRAINT achievements_type_check
    CHECK (type IN ('certificate', 'certification', 'webinar', 'seminar'));
