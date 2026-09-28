-- Profile data (2026-09-29): the last entities still served by Payload/Mongo.
--   profile.personal_info — singleton row (id = 1), the identity block.
--   profile.skills / experiences / projects / education — ordered lists.
-- Seeded from https://backend.abuamar.online/api/v1/personal/data so the
-- frontend can drop the CMS entirely. Guard makes it idempotent.

CREATE SCHEMA IF NOT EXISTS profile;

CREATE TABLE IF NOT EXISTS profile.personal_info (
    id         INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    fullname   VARCHAR(160) NOT NULL DEFAULT '',
    nickname   VARCHAR(80)  NOT NULL DEFAULT '',
    title      VARCHAR(120) NOT NULL DEFAULT '',
    email      VARCHAR(160) NOT NULL DEFAULT '',
    phone      VARCHAR(40)  NOT NULL DEFAULT '',
    location   VARCHAR(160) NOT NULL DEFAULT '',
    github     VARCHAR(300) NOT NULL DEFAULT '',
    linkedin   VARCHAR(300) NOT NULL DEFAULT '',
    instagram  VARCHAR(300) NOT NULL DEFAULT '',
    whatsapp   VARCHAR(300) NOT NULL DEFAULT '',
    website    VARCHAR(300) NOT NULL DEFAULT '',
    about_id   TEXT NOT NULL DEFAULT '',
    about_en   TEXT NOT NULL DEFAULT '',
    bio_id     TEXT NOT NULL DEFAULT '',
    bio_en     TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS profile.skills (
    id         SERIAL PRIMARY KEY,
    name       VARCHAR(120) NOT NULL,
    category   VARCHAR(40)  NOT NULL DEFAULT '',
    level      VARCHAR(40)  NOT NULL DEFAULT '',
    order_index INT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS profile.experiences (
    id           SERIAL PRIMARY KEY,
    company      VARCHAR(200) NOT NULL,
    position     VARCHAR(200) NOT NULL DEFAULT '',
    duration     VARCHAR(120) NOT NULL DEFAULT '',
    description  TEXT[] NOT NULL DEFAULT '{}'::text[],
    technologies TEXT[] NOT NULL DEFAULT '{}'::text[],
    order_index  INT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS profile.projects (
    id           SERIAL PRIMARY KEY,
    title        VARCHAR(200) NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    technologies TEXT[] NOT NULL DEFAULT '{}'::text[],
    github_url   VARCHAR(400) NOT NULL DEFAULT '',
    order_index  INT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS profile.education (
    id          SERIAL PRIMARY KEY,
    institution VARCHAR(200) NOT NULL,
    degree      VARCHAR(160) NOT NULL DEFAULT '',
    field       VARCHAR(200) NOT NULL DEFAULT '',
    duration    VARCHAR(120) NOT NULL DEFAULT '',
    gpa         VARCHAR(20)  NOT NULL DEFAULT '',
    order_index INT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_profile_skills_order ON profile.skills (order_index);
CREATE INDEX IF NOT EXISTS idx_profile_experiences_order ON profile.experiences (order_index);
CREATE INDEX IF NOT EXISTS idx_profile_projects_order ON profile.projects (order_index);
CREATE INDEX IF NOT EXISTS idx_profile_education_order ON profile.education (order_index);

INSERT INTO profile.personal_info (fullname, nickname, title, email, phone, location, github, linkedin, instagram, whatsapp, website, about_id, about_en, bio_id, bio_en)
SELECT * FROM (VALUES (
  'M. Abu Amar Al Badawi, S.Kom', 'Abu Amar', 'Software Engineer', 'abuamar.albadawi@gmail.com', '+6285117692402', 'Bantul, Yogyakarta, Indonesia', 'https://github.com/abuamar142', 'https://linkedin.com/in/abu-amar', 'https://instagram.com/abuuamar_', 'https://wa.me/6285117692402', 'https://abuamar.online', 'Software Engineer yang berbasis di Bantul, Yogyakarta. Saya lulus Informatika (S.Kom, IPK 3,90) dari Universitas Jenderal Achmad Yani Yogyakarta pada Oktober 2025. Saya bekerja sebagai Fullstack Javascript Developer di Berijalan (React Native, Javascript, TypeScript, Next.js) — intern sejak September 2025, freelance sejak Desember 2025 — serta sebagai Mobile Engineer full-time di Ikavia Digital Nusantara sejak Mei 2026 dengan Flutter. Sebelumnya intern Full-Stack di Refactory dan lulusan Bangkit Academy jalur Mobile Development. Saya merilis produk ke produksi, dari model data hingga deployment, dengan Linux harian.', 'Software Engineer based in Bantul, Yogyakarta. I graduated in Informatics (S.Kom, GPA 3.90) from Universitas Jenderal Achmad Yani Yogyakarta in October 2025. I work as a Fullstack Javascript Developer at Berijalan (React Native, Javascript, TypeScript, Next.js) — intern from September 2025, freelance since December 2025 — and as a full-time Mobile Engineer at Ikavia Digital Nusantara since May 2026, building with Flutter. Previously a Full-Stack intern at Refactory and a Bangkit Academy Mobile Development graduate. I ship products to production, from data model to deployment, on Linux daily.', 'Software Engineer yang membangun produk mobile dan web secara end to end: aplikasi Flutter, frontend React Native dan Next.js, beserta backend dan infrastruktur di belakangnya.', 'Software Engineer building mobile and web products end to end: Flutter apps, React Native and Next.js frontends, and the backend and infrastructure behind them.')
) AS v(fullname, nickname, title, email, phone, location, github, linkedin, instagram, whatsapp, website, about_id, about_en, bio_id, bio_en)
WHERE NOT EXISTS (SELECT 1 FROM profile.personal_info LIMIT 1);

INSERT INTO profile.skills (name, category, level, order_index)
SELECT * FROM (VALUES
  ('Machine Learning', 'tools', 'intermediate', 0),
  ('API Contract (Postman/Swagger)', 'tools', 'advanced', 1),
  ('C4 Model Documentation', 'tools', 'advanced', 2),
  ('OOP & Design Patterns', 'tools', 'advanced', 3),
  ('Google Sheets / Excel', 'tools', 'expert', 4),
  ('Linux', 'tools', 'expert', 5),
  ('Vercel', 'tools', 'advanced', 6),
  ('GitHub Actions (CI/CD)', 'tools', 'advanced', 7),
  ('Git & GitHub', 'tools', 'expert', 8),
  ('Database Schema Design', 'backend', 'expert', 9),
  ('Supabase', 'backend', 'expert', 10),
  ('API Integration (REST)', 'web', 'expert', 11),
  ('Bootstrap', 'web', 'advanced', 12),
  ('Tailwind CSS', 'web', 'expert', 13),
  ('CSS3', 'web', 'expert', 14),
  ('HTML5', 'web', 'expert', 15),
  ('TypeScript', 'web', 'advanced', 16),
  ('JavaScript', 'web', 'advanced', 17),
  ('Vue.js', 'web', 'advanced', 18),
  ('React', 'web', 'advanced', 19),
  ('Android', 'mobile', 'advanced', 20),
  ('Kotlin', 'mobile', 'advanced', 21),
  ('Clean Architecture', 'mobile', 'expert', 22),
  ('Riverpod', 'mobile', 'advanced', 23),
  ('BLoC', 'mobile', 'expert', 24),
  ('GetX', 'mobile', 'expert', 25),
  ('Dart', 'mobile', 'expert', 26),
  ('Flutter', 'mobile', 'expert', 27)
) AS v(name, category, level, order_index)
WHERE NOT EXISTS (SELECT 1 FROM profile.skills LIMIT 1);

INSERT INTO profile.experiences (company, position, duration, description, technologies, order_index)
SELECT * FROM (VALUES
  ('Ikavia Digital Nusantara', 'Mobile Engineer', '05/2026 - Present', '{"Full-time Mobile Engineer: mengembangkan aplikasi mobile dengan Flutter","Mengerjakan web dengan vanilla Javascript"}'::text[], '{"Flutter","Dart","Javascript"}'::text[], 0),
  ('Berijalan', 'Fullstack Javascript Developer (Freelance)', '12/2025 - Present', '{"Mengerjakan project IDMS dari Setir Kanan: aplikasi mobile dengan React Native","Mengerjakan project ACC Bid dari ACC: fullstack Javascript dan mobile React Native","Mengerjakan project Teman Seva dari Seva: fullstack Javascript"}'::text[], '{"React Native","Javascript","TypeScript","Next.js"}'::text[], 1),
  ('Berijalan', 'Fullstack Javascript Developer (Internship)', '09/2025 - 12/2025', '{"Mengerjakan project IDMS dari Setir Kanan: aplikasi mobile dengan React Native","Mempelajari Fullstack JavaScript, Spring Boot & Angular, React Native, serta System Design dan PLSQL","Terbiasa menggunakan workflow Git dan kolaborasi tim selama masa internship"}'::text[], '{"React Native","Javascript","TypeScript","Spring Boot","Angular","System Design","PLSQL"}'::text[], 2),
  ('PERMIKOMNAS Wilayah 8 - Yogyakarta', 'Bendahara Wilayah', '2023 - 2024', '{"Menangani pengelolaan keuangan organisasi di tingkat wilayah secara transparan dan bertanggung jawab","Menjadi penanggung jawab atas pelaksanaan program kerja Divisi BPD selama menjabat sebagai bendahara","Membuat laporan keuangan kegiatan dan memastikan efisiensi penggunaan dana pada berbagai program wilayah","Mengelola proyek produksi lanyard, ID card, dan jaket PERMIKOMNAS dari tahap perencanaan hingga distribusi"}'::text[], '{"Google Sheets","Financial Management","Project Management"}'::text[], 3),
  ('Bangkit Academy led by Google, GoTo, & Traveloka', 'Mobile Development Cohort', '02/2024 - 07/2024', '{"Mengikuti program pelatihan intensif selama 6 bulan dengan fokus pada Mobile Development (Android) menggunakan Kotlin dan Jetpack Compose","Mendalami konsep software engineering, UI/UX design, dan cloud computing sebagai bagian dari kurikulum interdisipliner","Mengembangkan proyek akhir berupa aplikasi deteksi penyakit tumbuhan kopi melalui image classification menggunakan teknologi AI","Berkolaborasi dalam tim multidisiplin (Mobile, Cloud, ML) untuk merancang dan membangun solusi nyata berbasis teknologi","Lulus sertifikasi internal Bangkit dan mendapatkan pengakuan langsung dari Google & Kampus Merdeka"}'::text[], '{"Kotlin","Jetpack Compose","Android","Machine Learning","Cloud Computing"}'::text[], 4),
  ('Refactory', 'Full Stack Software Engineer (Magang)', '07/2024 - 05/2025', '{"Mengikuti program untuk pengembangan proyek full-stack, mulai dari perencanaan hingga deployment","Menyusun dokumentasi teknis seperti C4 Model, database schema, dan API contract untuk kebutuhan project","Mengembangkan aplikasi mobile (Flutter), web (React), dan backend (Supabase dan Raiden) sesuai standar industri","Mengimplementasikan CI/CD pipeline menggunakan GitHub Actions untuk otomatisasi proses build dan deploy","Mengimplementasikan praktik software engineering mulai dari penggunaan OOP, algoritma, dan design pattern","Bekerja sama membuat proyek dengan tim menggunakan tools seperti GitHub dan workboard"}'::text[], '{"Flutter","React","Supabase","GitHub Actions","Linux","OOP"}'::text[], 5)
) AS v(company, position, duration, description, technologies, order_index)
WHERE NOT EXISTS (SELECT 1 FROM profile.experiences LIMIT 1);

INSERT INTO profile.projects (title, description, technologies, github_url, order_index)
SELECT * FROM (VALUES
  ('Plasma Token Usage', 'Widget KDE Plasma 6 system tray untuk memantau pemakaian token AI coding agent (berbasis ccusage). Ditulis dengan QML untuk desktop Linux.', '{"QML","KDE Plasma 6","Linux"}'::text[], 'https://github.com/abuamar142/plasma-tokenusage', 0),
  ('LMS Bahasa Arab', 'Aplikasi LMS pengganti LKS untuk mata pelajaran Bahasa Arab kelas 7. Dibangun dengan Flutter dan GetX: materi markdown, file picker, dan penyimpanan lokal.', '{"Flutter","Dart","GetX","Android"}'::text[], 'https://github.com/abuamar142/lms', 1),
  ('Warloc - Chat Backup Viewer', 'WhatsApp dan Telegram chat backup and viewer untuk Android. Dibangun dengan Flutter: sqflite untuk penyimpanan lokal, file picker dan archive untuk impor backup, plus pemutar media.', '{"Flutter","Dart","sqflite","Android"}'::text[], 'https://github.com/abuamar142/warloc', 2),
  ('Hafalan - Quran Tracker', 'Hafalan Quran Tracker: catat hafalan dan setoran santri. Dibangun dengan Next.js, Supabase, dan TanStack Query, live di Vercel.', '{"TypeScript","Next.js","Supabase","TanStack Query","Tailwind CSS"}'::text[], 'https://github.com/abuamar142/hafalan', 3),
  ('Tambangan - Perahu Tracker', 'Aplikasi pelacakan perahu tambangan real-time. Live di tambangan.abuamar.online dengan stack TypeScript, Next.js, Drizzle ORM, dan PostgreSQL.', '{"TypeScript","Next.js","Drizzle ORM","PostgreSQL","Docker"}'::text[], 'https://github.com/abuamar142/tambangan', 4),
  ('Liat Menu - Restaurant Platform', 'Platform web modern untuk discovery warung dan restoran menggunakan Next.js dan Supabase. Fitur pencarian, filter kategori, review sistem, autentikasi pengguna, dashboard admin, dan responsive design. Dilengkapi dengan dark/light mode dan real-time data.', '{"Next.js","TypeScript","Supabase","Tailwind CSS","shadcn/ui"}'::text[], 'https://github.com/abuamar142/liat-menu', 5),
  ('MonkeyType CLI', 'Aplikasi command-line interface berbasis Python untuk latihan typing speed test. Dioptimalkan khusus untuk Windows dengan interface yang user-friendly dan tracking progress untuk meningkatkan kecepatan dan akurasi mengetik.', '{"Python","CLI","Windows Optimization","Performance Tracking"}'::text[], 'https://github.com/abuamar142/monkeytype-cli', 6),
  ('N8N Local Automation Setup', 'Setup Docker-based N8N workflow automation dengan konfigurasi Ngrok untuk tunneling lokal. Menyediakan environment automation lengkap untuk development dan testing workflow dengan akses eksternal yang aman.', '{"Docker","N8N","Ngrok","Workflow Automation","Shell Scripts"}'::text[], 'https://github.com/abuamar142/n8n-local', 7),
  ('Agenda - Google Calendar Manager', 'Aplikasi Flutter manajemen agenda terintegrasi dengan Google Calendar. Menggunakan Supabase OAuth untuk autentikasi, GetX untuk state management, dan clean architecture. Memungkinkan sinkronisasi seamless dengan Google Calendar untuk pengelolaan jadwal yang efektif.', '{"Flutter","Google Calendar API","Supabase","GetX","Clean Architecture"}'::text[], 'https://github.com/abuamar142/agenda', 8),
  ('Inget - Note Taking App', 'Aplikasi pencatatan modern berbasis Flutter dengan state management Riverpod. Fitur offline storage, Material Design yang clean, dan interface yang intuitif untuk produktivitas maksimal dalam mencatat dan mengorganisir informasi.', '{"Flutter","Riverpod","Material Design","Offline Storage","State Management"}'::text[], 'https://github.com/abuamar142/inget', 9),
  ('My SADARI - Breast Self-Examination App', 'Aplikasi Flutter untuk edukasi dan pengingat pemeriksaan payudara sendiri (SADARI). Dilengkapi dengan fitur notifikasi terjadwal, kuesioner penilaian, tutorial step-by-step, dan sistem autentikasi. Membantu meningkatkan awareness kesehatan wanita untuk deteksi dini kanker payudara.', '{"Flutter","Dart","Firebase","Local Notifications","Authentication"}'::text[], 'https://github.com/abuamar142/my_sadari', 10),
  ('Cashier App', 'Aplikasi kasir (Point of Sale) berbasis Flutter untuk manajemen penjualan, pembelian, inventori, dan laporan keuangan. Dilengkapi dengan fitur draft penjualan, manajemen produk, supplier, dan sistem autentikasi pengguna.', '{"Flutter","Dart","GetX","REST API","Database Management"}'::text[], 'https://github.com/abuamar142/cashier_app', 11),
  ('Pondok Pesantren Asy-Syaikhoni', 'Website landing page modern dan elegan untuk Pondok Pesantren Tahfidzul Qur''an Asy-Syaikhoni di Nganjuk, Jawa Timur. Dibangun dengan desain islami menggunakan Vue.js 3 dan Tailwind CSS v4.', '{"Vue.js","TypeScript","Tailwind CSS","Vite","SEO Optimization","Vercel"}'::text[], 'https://github.com/abuamar142/asyaikhoni', 12),
  ('JNE Landing Page', 'Landing page modern dan responsif untuk JNE Express - perusahaan ekspedisi pengiriman barang terpercaya di Indonesia.', '{"Vue.js","Tailwind CSS","TypeScript","SEO Optimization","Vercel"}'::text[], 'https://github.com/abuamar142/jne-landing-page', 13),
  ('Coffee Plant Disease Detection App', 'Aplikasi mobile untuk deteksi penyakit tumbuhan kopi menggunakan image classification dan machine learning. Dikembangkan sebagai proyek akhir Bangkit Academy dengan kolaborasi tim multidisiplin.', '{"Kotlin","Jetpack Compose","TensorFlow","Android","Machine Learning"}'::text[], 'https://github.com/abuamar142/coffeeClassifier', 14),
  ('Mobile Londri', 'Aplikasi mobile manajemen laundry berbasis Flutter dan Supabase. Mendukung fitur pesanan, pelanggan, laporan keuangan, dan autentikasi pengguna.', '{"Flutter","Supabase","Dart","PostgreSQL"}'::text[], 'https://github.com/abuamar142/mobile-londri', 15)
) AS v(title, description, technologies, github_url, order_index)
WHERE NOT EXISTS (SELECT 1 FROM profile.projects LIMIT 1);

INSERT INTO profile.education (institution, degree, field, duration, gpa, order_index)
SELECT * FROM (VALUES
  ('Universitas Jenderal Achmad Yani Yogyakarta', 'Sarjana (S1)', 'Informatika', '2021 - 2025', '3.90', 0),
  ('Pondok Pesantren Al-Munawwir Krapyak', 'Pendidikan Agama', 'Komplek Madrasah Huffadh 2', '2021 - Saat ini', '', 1),
  ('Pondok Pesantren Al-Munawwir Krapyak', 'Pendidikan Agama', 'Komplek Madrasah Huffadh 2', '2021 - 2024', '', 2),
  ('MAN 2 Kabupaten Kediri', 'SMA', 'Kelas PDCI (Peserta Didik Cerdas Istimewa)', '2017 - 2019', 'Lulus 2 tahun', 3)
) AS v(institution, degree, field, duration, gpa, order_index)
WHERE NOT EXISTS (SELECT 1 FROM profile.education LIMIT 1);
