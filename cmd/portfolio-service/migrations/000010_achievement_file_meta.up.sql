-- Achievement file metadata (2026-09-28).
--   Adds file_name and file_size columns for client-provided file metadata,
--   backfills file_key for the 20 seed rows from legacy Drive IDs,
--   populates file_name from title slug, and drops the legacy drive_file_id column.
--   file_key is made nullable to support clearing on file deletion.

-- a. Add new nullable columns.
ALTER TABLE achievements.achievements ADD COLUMN file_name VARCHAR(255);
ALTER TABLE achievements.achievements ADD COLUMN file_size BIGINT;

-- Allow file_key to be NULL (cleared when file is deleted).
ALTER TABLE achievements.achievements ALTER COLUMN file_key DROP NOT NULL;
ALTER TABLE achievements.achievements ALTER COLUMN file_key DROP DEFAULT;

-- b. Backfill file_key for the 20 seed rows from legacy Drive IDs.
UPDATE achievements.achievements
SET file_key = 'certificates/' || v.fid || '.pdf'
FROM (VALUES
    ('Belajar Fundamental Aplikasi Android','2024-03-28'::date,'10FU3FL7_uLP_vBzm41QxdwI6Y9yWhyrz'),
    ('Belajar Dasar Git dengan GitHub','2024-03-06'::date,'16kgtgCMvOn5PzVopU82n6JqR86XVir_-'),
    ('Kepanitiaan Permikomnas Festival 2022','2022-11-26'::date,'1BT0IktahCWY7bSKXePxyK4Lpt3Bjdjld'),
    ('Tips & Trik Buat Aplikasi Tanpa Coding','2025-03-08'::date,'1r1AjC0_RfF058luxQwFsi6KH4Ez5ecci'),
    ('Melihat Dunia dengan AI : Eksplorasi Computer Vision','2024-07-19'::date,'1PzJCEZmTU0-V5yDQvD3cPzs6ADzqVTq2'),
    ('Kepanitiaan Permikomnas Festival 2023','2023-10-01'::date,'1DoisKRbgPGs_z2Au02e23c4wuq9W9jJW'),
    ('English for Business Communication','2024-07-12'::date,'1PdN0LT49jqUlbx9425gnfwM0TqsNbUJo'),
    ('Belajar Dasar AI','2024-04-11'::date,'1vlI6MuiOJ-BTU458rsU9a4j3C7M0ZcvU'),
    ('Cybersecurity & Artificial Intelligence : Towards an Epoch of Digital Vigilance','2023-10-01'::date,'1IwSJUDryd9EuqM-tAseItGenq4ptNa9H'),
    ('Pelatihan Literasi Digital, Finansial, & Dampak Sosial','2022-12-03'::date,'12cma6oqNkJXmIq3KQwgad-37BPgZES-W'),
    ('Sertifikat Kepengurusan Permikomnas Wilayah VIII Yogyakarta','2024-12-31'::date,'1ZkZF1Y2jkAHfOLW7MeL2lbq6g138Rf_7'),
    ('Membuat Mobile Aplikasi dengan Efisien dan Cepat','2025-02-22'::date,'10_K4se5WiVm4IzBy2mk812gn49BhtiXN'),
    ('Sertifikat Kepengurusan Permikomnas Wilayah VIII Yogyakarta','2023-03-20'::date,'1BoPMX74S3Ptv-uwFBn2zBf00Dx6dDBAR'),
    ('Graduation Certificate - Bangkit 2024 (Mobile Development - Android)','2024-07-10'::date,'1Qq23JBni7tGN-q9FZ9W5s2vOEBBQQSJ8'),
    ('Memulai Pemrograman dengan Kotlin','2024-02-28'::date,'1vAN-TOthSKQO9muxv80KWzvobYuAt-UY'),
    ('Memulai Dasar Pemrograman untuk Menjadi Pengembang Software','2024-02-21'::date,'1wuRg5fiulMKw-WU5RojbDAq475vGBt8_'),
    ('Belajar Prinsip Pemrograman SOLID','2024-04-07'::date,'1LhEQdIZVcWGavXdOuq_Wzuyw9lgGPQf0'),
    ('Belajar Pengembangan Aplikasi Android Intermediate','2024-05-31'::date,'1pACgV-jjnvt2X2IAPuLz3CO77dTZhBQ2'),
    ('Belajar Penerapan Machine Learning untuk Android','2024-04-25'::date,'193ESxFL96Fk6dBF8l53jQYklrxb-MxgR'),
    ('Belajar Membuat Aplikasi Android untuk Pemula','2024-03-05'::date,'155xRT6enJEu9oKkQI9wrbRn3-ugi0vFt')
) AS v(title, date, fid)
WHERE achievements.title = v.title
  AND achievements.date = v.date::date
  AND (achievements.file_key = '' OR achievements.file_key IS NULL);

-- c. Populate file_name from title slug for rows that have a file_key.
UPDATE achievements.achievements
SET file_name = lower(regexp_replace(title, '[^a-zA-Z0-9]+', '-', 'g')) || '.pdf'
WHERE file_name IS NULL
  AND file_key IS NOT NULL
  AND file_key <> '';

-- d. Drop the legacy Google Drive file ID column.
ALTER TABLE achievements.achievements DROP COLUMN drive_file_id;
