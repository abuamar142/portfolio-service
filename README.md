# portfolio-service

Backend API untuk situs portfolio `abuamar.online` (Go + chi + PostgreSQL). Melayani profil, kutipan, tautan, cuplikan kode, pencapaian, feedback, dan game realtime kecil.

**Stack:** Go 1.25 · chi v5 · PostgreSQL · notifikasi Telegram

---

## Arsitektur

```
cmd/
  portfolio-service/  — entry point HTTP server
internal/
  config/             — konfigurasi dari env
  db/                 — koneksi Postgres
  game/               — logika game realtime (room, player, event)
  handlers/           — HTTP handler per resource
  middleware/         — CORS, auth owner, request ID, logger
  models/             — model GORM: achievement, feedback, link, profile, quote, snippet
  notify/             — notifikasi Telegram
  response/           — envelope JSON standar
  services/           — business logic
```

## Endpoint utama

Semua di bawah prefix `/api/v1`.

| Resource | Endpoint |
|---|---|
| Profil | `GET /profile` |
| Kutipan | `GET /quotes` · `GET /quotes/tags` · `GET /quotes/{id}` · `DELETE /quotes/{id}` |
| Tautan | `GET /links` · `GET /links/tags` · `GET /links/{id}` · `DELETE /links/{id}` |
| Cuplikan kode | `GET /snippets` · `GET /snippets/languages` · `GET /snippets/tags` · `GET /snippets/{id}` · `DELETE /snippets/{id}` |
| Pencapaian | `GET /achievements` · `DELETE /achievements/{id}` · `DELETE /achievements/{id}/file` |
| Feedback | `GET /feedback` · `DELETE /feedback/{id}` |
| Game | `GET /game/rooms/{code}` · `GET /game/rooms/{code}/events` · `DELETE /game/rooms/{code}/players/{playerID}` |
| Health | `GET /api/health` |

Endpoint `DELETE` dijaga middleware auth owner.

## Konfigurasi

| Variabel | Keterangan |
|---|---|
| `DATABASE_URL` | DSN Postgres |
| `ALLOWED_ORIGINS` | Origin CORS yang diizinkan |
| `OWNER_USER_ID` | Identitas owner untuk endpoint tulis |
| `TELEGRAM_BOT_TOKEN` | Token bot untuk notifikasi |
| `TELEGRAM_CHAT_ID` | Chat tujuan notifikasi |

## Menjalankan lokal

```bash
docker compose up --build
```

Atau tanpa Docker: siapkan Postgres, set `DATABASE_URL`, lalu `go run ./cmd/portfolio-service`.

## Deploy

Produksi jalan di VPS `abuamar.online` (`/opt/portfolio-service`).

| Branch | Environment | Domain | Upstream |
|---|---|---|---|
| `main` | prod | `portfolio.abuamar.online` | `127.0.0.1:8084` |
| `development` | dev | `portfolio-dev.abuamar.online` | `127.0.0.1:8088` |

Skrip: `/opt/ops/bin/deploy-portfolio-service.sh <branch>`.

> **Jebakan:** `/opt/portfolio-service-dev` bukan repo — hanya compose + `.env`, dan build context-nya menunjuk ke `/opt/portfolio-service`. Artinya prod dan dev **berbagi satu source dir**; dua deploy bersamaan akan saling menimpa. Skrip memakai `flock` untuk mencegahnya — jangan dihapus.

**Cara deploy:** push ke `main` → timer `pull-deploy.timer` di VPS menarik ref tiap 5 menit dan menjalankan skrip di atas. GitHub Actions tidak dipakai (akun terkunci billing).

## Terkait

- `portfolio` — frontend Vue 3 yang mengonsumsi API ini
