-- Achievements & certificates (2026-09-27).
--   achievements.* — portfolio achievement cards (certificates, seminars,
--   webinars), ported one-shot from the Payload/Mongo achievements collection
--   (/tmp export of 20 rows; guard below skips if the table already has data).
--   drive_file_id = legacy Google Drive file link; file_key = R2 object key in
--   the portfolio-assets bucket, filled as files move off Drive.

CREATE SCHEMA IF NOT EXISTS achievements;

CREATE TABLE IF NOT EXISTS achievements.achievements (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title              VARCHAR(255) NOT NULL,
    organizer          VARCHAR(255) NOT NULL DEFAULT '',
    date               DATE NOT NULL,
    type               VARCHAR(20) NOT NULL CHECK (type IN ('certificate', 'certification', 'webinar', 'seminar')),
    drive_file_id      VARCHAR(120) NOT NULL DEFAULT '',
    file_key           VARCHAR(300) NOT NULL DEFAULT '',
    certificate_number VARCHAR(100) NOT NULL DEFAULT '',
    participant_as     VARCHAR(160) NOT NULL DEFAULT '',
    description        TEXT NOT NULL DEFAULT '',
    valid_until        DATE,
    order_index        INT NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_achievements_order ON achievements.achievements (order_index, created_at);
CREATE INDEX IF NOT EXISTS idx_achievements_type ON achievements.achievements (type);
INSERT INTO achievements.achievements (title, organizer, date, type, drive_file_id, certificate_number, participant_as, description, valid_until, order_index, created_at)
SELECT * FROM (VALUES
('Belajar Fundamental Aplikasi Android','Dicoding | Bangkit collaborates with Google, Tokopedia, and Traveloka','2024-03-28'::date,'certificate','10FU3FL7_uLP_vBzm41QxdwI6Y9yWhyrz','53XEYVY4YPRN','None','Kelulusan kelas','2027-03-28'::date,0,'2025-08-03T01:55:28.276Z'::timestamptz),
('Belajar Dasar Git dengan GitHub','Dicoding | Bangkit collaborates with Google, Tokopedia, and Traveloka','2024-03-06'::date,'certificate','16kgtgCMvOn5PzVopU82n6JqR86XVir_-','MRZM818WNZYQ','None','Kelulusan kelas','2027-03-06'::date,1,'2025-08-03T01:55:28.379Z'::timestamptz),
('Kepanitiaan Permikomnas Festival 2022','Permikomnas Wilayah XIII Yogyakarta','2022-11-26'::date,'seminar','1BT0IktahCWY7bSKXePxyK4Lpt3Bjdjld','13/237/PERMIKOMNASYK/XI/2022','Anggota Divisi Perlengkapan','None',NULL,2,'2025-08-03T01:55:28.480Z'::timestamptz),
('Tips & Trik Buat Aplikasi Tanpa Coding','Devcode AI Talks','2025-03-08'::date,'webinar','1r1AjC0_RfF058luxQwFsi6KH4Ez5ecci','026/AIT-DEV/III/25','Participant','No-Code & AI',NULL,3,'2025-08-03T01:55:28.504Z'::timestamptz),
('Melihat Dunia dengan AI : Eksplorasi Computer Vision','Dicoding Event','2024-07-19'::date,'webinar','1PzJCEZmTU0-V5yDQvD3cPzs6ADzqVTq2','None','Peserta','DevCoach 160 : Machine Learning',NULL,4,'2025-08-03T01:55:28.583Z'::timestamptz),
('Kepanitiaan Permikomnas Festival 2023','Permikomnas Wilayah XIII Yogyakarta','2023-10-01'::date,'seminar','1DoisKRbgPGs_z2Au02e23c4wuq9W9jJW','016/FST/UPY/X/2023','Panitia','Seminar Nasional berlokasi di Universitas PGRI Yogyakarta (UPY)',NULL,5,'2025-08-03T01:55:28.606Z'::timestamptz),
('English for Business Communication','The British Institute | Bangkit collaborates with Google, Tokopedia, and Traveloka','2024-07-12'::date,'certificate','1PdN0LT49jqUlbx9425gnfwM0TqsNbUJo','TBI-DAGO/CORP/9957','None','Completed a short course and achieved an overall score of 76%',NULL,6,'2025-08-03T01:55:28.686Z'::timestamptz),
('Belajar Dasar AI','Dicoding | Bangkit collaborates with Google, Tokopedia, and Traveloka','2024-04-11'::date,'certificate','1vlI6MuiOJ-BTU458rsU9a4j3C7M0ZcvU','07Z60LWRJZQR','None','Kelulusan kelas','2027-04-11'::date,7,'2025-08-03T01:55:28.710Z'::timestamptz),
('Cybersecurity & Artificial Intelligence : Towards an Epoch of Digital Vigilance','Permikomnas Wilayah XIII Yogyakarta','2023-10-01'::date,'seminar','1IwSJUDryd9EuqM-tAseItGenq4ptNa9H','015/FST/UPY/X/2023','Peserta','Seminar Nasional berlokasi di Universitas PGRI Yogyakarta (UPY)',NULL,8,'2025-08-03T01:55:28.740Z'::timestamptz),
('Pelatihan Literasi Digital, Finansial, & Dampak Sosial','Digital & Financial Literacy Festival collaborates with Traveloka','2022-12-03'::date,'certificate','12cma6oqNkJXmIq3KQwgad-37BPgZES-W','606/TRVD/XII/2022','Peserta','Bertempat di Dinas Koperasi & UMKM DIY',NULL,9,'2025-08-03T01:55:28.788Z'::timestamptz),
('Sertifikat Kepengurusan Permikomnas Wilayah VIII Yogyakarta','Permikomnas Wilayah VIII Yogyakarta','2024-12-31'::date,'certificate','1ZkZF1Y2jkAHfOLW7MeL2lbq6g138Rf_7','067/SERT.e/K8-PERMIKOMNAS/V/2024','Bendahara Wilayah','Dalam satu periode kepengurusan dengan masa periode 2023 - 2024',NULL,10,'2025-08-03T01:55:28.810Z'::timestamptz),
('Membuat Mobile Aplikasi dengan Efisien dan Cepat','HMIF UNJAYA','2025-02-22'::date,'certificate','10_K4se5WiVm4IzBy2mk812gn49BhtiXN','S/01/HMIF.UNJAYA/02/2025','Peserta','Workshop HMIF 2025',NULL,11,'2025-08-03T01:55:28.893Z'::timestamptz),
('Sertifikat Kepengurusan Permikomnas Wilayah VIII Yogyakarta','Permikomnas Wilayah VIII Yogyakarta','2023-03-20'::date,'certificate','1BoPMX74S3Ptv-uwFBn2zBf00Dx6dDBAR','13/142/PERMIKOMNASYK/III/2023','Anggota Divisi Bisnis, Pemasaran, dan Distribusi','Dalam satu periode kepengurusan dengan masa periode 2021 - 2022',NULL,12,'2025-08-03T01:55:28.918Z'::timestamptz),
('Graduation Certificate - Bangkit 2024 (Mobile Development - Android)','Bangkit by Google, in collaboration with GoTo, Tokopedia, Traveloka','2024-07-10'::date,'certificate','1Qq23JBni7tGN-q9FZ9W5s2vOEBBQQSJ8','BA24/GRAD/XXIV-07/A228D4KY3895','Student','This certificate confirms that the participant has successfully graduated from Bangkit 2024 Batch 1, following the Mobile Development (Android) learning path',NULL,13,'2025-08-03T01:55:28.993Z'::timestamptz),
('Memulai Pemrograman dengan Kotlin','Dicoding | Bangkit collaborates with Google, Tokopedia, and Traveloka','2024-02-28'::date,'certificate','1vAN-TOthSKQO9muxv80KWzvobYuAt-UY','L4PQQL6V4PO1','None','Kelulusan kelas','2027-02-28'::date,14,'2025-08-03T01:55:29.018Z'::timestamptz),
('Memulai Dasar Pemrograman untuk Menjadi Pengembang Software','Dicoding | Bangkit collaborates with Google, Tokopedia, and Traveloka','2024-02-21'::date,'certificate','1wuRg5fiulMKw-WU5RojbDAq475vGBt8_','MRZMELJDKPYQ','None','Kelulusan kelas','2027-02-21'::date,15,'2025-08-03T01:55:29.096Z'::timestamptz),
('Belajar Prinsip Pemrograman SOLID','Dicoding | Bangkit collaborates with Google, Tokopedia, and Traveloka','2024-04-07'::date,'certificate','1LhEQdIZVcWGavXdOuq_Wzuyw9lgGPQf0','GRX5ONVL3P0M','None','Kelulusan kelas','2027-04-07'::date,16,'2025-08-03T01:55:29.120Z'::timestamptz),
('Belajar Pengembangan Aplikasi Android Intermediate','Dicoding | Bangkit collaborates with Google, Tokopedia, and Traveloka','2024-05-31'::date,'certificate','1pACgV-jjnvt2X2IAPuLz3CO77dTZhBQ2','4EXGQ794DZRL','None','Kelulusan kelas','2027-05-31'::date,17,'2025-08-03T01:55:29.198Z'::timestamptz),
('Belajar Penerapan Machine Learning untuk Android','Dicoding | Bangkit collaborates with Google, Tokopedia, and Traveloka','2024-04-25'::date,'certificate','193ESxFL96Fk6dBF8l53jQYklrxb-MxgR','4EXGQ794DZRL','None','Kelulusan kelas','2027-04-25'::date,18,'2025-08-03T01:55:29.220Z'::timestamptz),
('Belajar Membuat Aplikasi Android untuk Pemula','Dicoding | Bangkit collaborates with Google, Tokopedia, and Traveloka','2024-03-05'::date,'certificate','155xRT6enJEu9oKkQI9wrbRn3-ugi0vFt','MRZM81DENZYQ','None','Kelulusan kelas','2027-03-05'::date,19,'2025-08-03T01:55:29.244Z'::timestamptz)
) AS v(title, organizer, date, type, drive_file_id, certificate_number, participant_as, description, valid_until, order_index, created_at)
WHERE NOT EXISTS (SELECT 1 FROM achievements.achievements LIMIT 1);