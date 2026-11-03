-- Reverse: the previous home base.
--   Patches the same place phrases back; it does not restore any other edit made
--   to the about paragraphs in the meantime.

UPDATE profile.personal_info
SET location = 'Bantul, Yogyakarta, Indonesia',
    about_id = replace(about_id, 'berbasis di Nganjuk, Jawa Timur', 'berbasis di Bantul, Yogyakarta'),
    about_en = replace(about_en, 'based in Nganjuk, East Java', 'based in Bantul, Yogyakarta');
