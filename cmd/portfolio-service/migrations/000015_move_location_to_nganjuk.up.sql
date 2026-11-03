-- Move the home base from Bantul to Nganjuk (2026-11-03).
--   Location is identity data, not a build constant: the hub, abuamar.online and
--   the generated CV all read it from this table, so changing it here changes
--   every surface at once.
--
--   The two about paragraphs open with the city, so they are patched rather than
--   rewritten: only the place phrase changes, every claim in them stays as
--   written. Timezone is not stored (it is the UI constant "WIB (UTC+7)" in the
--   frontend), and Nganjuk is also UTC+7, so nothing else moves.

UPDATE profile.personal_info
SET location = 'Nganjuk, Jawa Timur, Indonesia',
    about_id = replace(about_id, 'berbasis di Bantul, Yogyakarta', 'berbasis di Nganjuk, Jawa Timur'),
    about_en = replace(about_en, 'based in Bantul, Yogyakarta', 'based in Nganjuk, East Java');
