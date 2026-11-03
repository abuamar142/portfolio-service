-- Drop repository links that cannot resolve for a visitor (2026-11-03).
--   Four seeded projects point at repositories that are PRIVATE:
--     liat-menu (now named cafe), my_sadari, cashier_app, mobile-londri
--   GitHub answers 404 for them, so every card carried a broken "Repo" link.
--   There is no public URL to substitute — the code is not open — so the link
--   is cleared instead of guessed. An empty github_url means "no repository
--   link", the same convention live_url uses.
--
--   If any of these repositories is made public later, set the URL back from
--   the dashboard; this migration only removes values that are known-broken.

UPDATE profile.projects
SET github_url = ''
WHERE github_url IN (
    'https://github.com/abuamar142/liat-menu',
    'https://github.com/abuamar142/my_sadari',
    'https://github.com/abuamar142/cashier_app',
    'https://github.com/abuamar142/mobile-londri'
);
