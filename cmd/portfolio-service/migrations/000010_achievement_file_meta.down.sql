-- Reverse achievement file metadata.
-- NOTE: drive_file_id is re-added empty; original Drive IDs are lost
-- and must be re-populated from the Mongo export if needed.

-- Restore drive_file_id column.
ALTER TABLE achievements.achievements ADD COLUMN drive_file_id VARCHAR(120) NOT NULL DEFAULT '';

-- Restore file_key constraints.
ALTER TABLE achievements.achievements ALTER COLUMN file_key SET NOT NULL;
ALTER TABLE achievements.achievements ALTER COLUMN file_key SET DEFAULT '';

-- Drop file metadata columns.
ALTER TABLE achievements.achievements DROP COLUMN file_name;
ALTER TABLE achievements.achievements DROP COLUMN file_size;
