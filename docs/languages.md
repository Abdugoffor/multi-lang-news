# Tillar API

Language moduli `handler -> service -> repository` orqali ishlaydi. Loyihada auth moduli mavjud emas; hozir ushbu route'lar autentifikatsiyasiz ishlaydi.

Repositoryning `List` metodi `[]lang_dto.Response` qaytaradi. Service shu slice'ni qayta nusxalamasdan `ListResponse.Data`ga uzatadi.

Repositoryning `HasMore(ctx, filter, cursor, direction)` metodi qarama-qarshi yo'nalishda shu filtrlarga mos yozuv mavjudligini `SELECT EXISTS` orqali tekshiradi va faqat `bool` qaytaradi. `ListActive` paginatsiyasiz ishlashda davom etadi.

| Metod | Manzil | Natija |
| --- | --- | --- |
| POST | `/api/languages` | Til yaratish, `201` |
| GET | `/api/languages` | Cursor pagination bilan ro'yxat, `200` |
| GET | `/api/languages-active` | Barcha aktiv tillar, paginatsiyasiz, `200` |
| GET | `/api/languages/:id` | Bitta til, `200` |
| PUT | `/api/languages/:id` | Tilni yangilash, `200` |
| DELETE | `/api/languages/:id` | Tilni o'chirish, bodysiz `204` |

Aktiv tillar route'i `/:id` bilan to'qnashmasligi uchun `/api/languages-active` manzilida joylashgan. Natija `id DESC` tartibidagi JSON massiv; til bo'lmasa `[]`.

Create va Update uchun body:

```json
{
  "name": "Uzbek",
  "is_active": true
}
```

`name` chetidagi bo'shliqlar olib tashlanadi; 2–10 belgidan iborat bo'lishi kerak. PUT to'liq yangilash uchun ishlatiladi. `is_active` yuborilmasa `false` bo'ladi. Body hajmi ko'pi bilan 1 MiB; noma'lum maydonlar va bir nechta JSON qiymati qabul qilinmaydi.

List filterlari:

| Query | Tavsif |
| --- | --- |
| `name` | Nom bo'yicha katta-kichik harflarni farqlamasdan qidirish |
| `is_active` | Faqat `true` yoki `false`; yuborilmasa barchasi |
| `limit` | 1–100, standart `10` |
| `direction` | `next` yoki `prev`, standart `next` |
| `cursor` | Musbat ID, `prev` uchun majburiy |

Misol: `GET /api/languages?limit=10&direction=next&is_active=true`.

```json
{
  "data": [],
  "next_cursor": null,
  "prev_cursor": null,
  "has_next": false,
  "has_prev": false
}
```

Ro'yxat har ikki yo'nalishda ham `id DESC` tartibida qaytariladi. Keyingi so'rovda javobdagi `next_cursor` yoki `prev_cursor` ishlatiladi; boshqa filterlar saqlanadi.

Noto'g'ri body, ID va query uchun `400`; til topilmasa `404`; FK cheklovi yoki parallel o'zgarish ziddiyatida `409` qaytariladi. Kutilmagan xato logga yoziladi va ichki tafsilotlarsiz `500` qaytariladi. Xatolar `{"error":"..."}`, DTO validatsiya xatolari `{"errors":{...}}` shaklida keladi.

## Katta jadval uchun so'rovlar

- `OFFSET` va umumiy `COUNT(*)` ishlatilmaydi. Cursor bo'yicha `id < cursor` yoki `id > cursor`, sahifa uchun ko'pi bilan `limit + 1` yozuv olinadi. `limit` ko'pi bilan `100`.
- Bo'sh filtrlar SQLga qo'shilmaydi; filtr mavjudligini tekshiruvchi parametrli `OR` shartlari yo'q. Foydalanuvchi qiymatlari `$1`, `$2` kabi parametrlar orqali uzatiladi.
- Birinchi `next` sahifada bitta `List` so'rovi bajariladi. Keyingi va oldingi sahifalarda ko'pi bilan bitta qo'shimcha `HasMore` so'rovi bajariladi. Qarama-qarshi sahifani tekshirish uchun to'liq DTOlar o'qilmaydi.
- `EXISTS` odatda birinchi mos yozuv topilganda tekshirishni tugatadi; mos yozuv bo'lmasa ko'proq indeks yoki jadval qatori tekshirilishi mumkin. [PostgreSQL hujjati](https://www.postgresql.org/docs/current/functions-subquery.html#FUNCTIONS-SUBQUERY-EXISTS).
- `005_languages_active_id_index.sql`: `languages (is_active, id DESC)` B-tree indeksi. Filtrsiz cursor uchun mavjud primary-key indeksi ham bor. [B-tree tartibi va LIMIT](https://www.postgresql.org/docs/current/indexes-ordering.html).
- `006_languages_name_trgm_index.sql`: `name gin_trgm_ops` GIN indeksi; `001_extensions.sql`dagi `pg_trgm`dan foydalanadi. `ILIKE '%...%'` qidiruvi saqlangan. Trigram chiqarib bo'lmaydigan qisqa yoki faqat wildcarddan iborat patternlar qimmat bo'lishi mumkin. [pg_trgm hujjati](https://www.postgresql.org/docs/current/pgtrgm.html#PGTRGM-INDEX).

Ikkala yangi migratsiya ham alohida, bitta `CREATE INDEX CONCURRENTLY` buyrug'idan iborat. Mavjud migratsiya mexanizmi ularni alohida `db.Exec` orqali bajaradi; ularni `BEGIN/COMMIT` ichiga o'ramang. Indeks qurilishi vaqtida oddiy yozish amallari davom etishi mumkin. Qurilish uzilib qolsa, qayta urinishdan oldin `pg_index.indisvalid` orqali indeks yaroqliligini tekshirish kerak; `IF NOT EXISTS` yaroqsiz indeksni qayta qurmaydi. [Concurrent index yaratish](https://www.postgresql.org/docs/current/sql-createindex.html#SQL-CREATEINDEX-CONCURRENTLY).

## Lokal yuklama tekshiruvi

2026-11-11 kuni izolyatsiyalangan PostgreSQL 17 Docker konteynerida 1 000 000 sun'iy qator bilan tekshirildi. Nomlar `lng0000001` ko'rinishida, har o'ninchi yozuv aktiv. Haqiqiy `LangService` va `LangRepository`, Unix socket ulanishi, pool limiti `50`, sahifa hajmi `10`, `32` parallel chaqiruv ishlatildi. Har ish yukida `3 200` so'rov bajarildi; `next` va `prev` navbatma-navbat chaqirildi.

| So'rov | So'rov/sekund | p50 | p95 | p99 |
| --- | --- | --- | --- | --- |
| Cursor | 1 679 | 7.401 ms | 72.003 ms | 110.615 ms |
| Cursor + aktivlik | 1 584 | 6.513 ms | 79.251 ms | 121.999 ms |
| Cursor + nomda `9999` qidirish | 1 229 | 24.270 ms | 39.242 ms | 48.254 ms |

15 ta pagination holati va 6 ta `HasMore` chegaraviy/filtr holati o'tdi: birinchi/oxirgi sahifa, `next`/`prev`, aktiv/aktiv bo'lmagan yozuvlar, nom qidiruvi, bo'sh natija va katta cursor tekshirildi. Jami 9 600 parallel service chaqiruvida xato kuzatilmadi. Vaqtinchalik tekshiruv dasturi va konteyner tekshiruvdan keyin olib tashlandi.

`EXPLAIN (ANALYZE, BUFFERS)` cursor so'rovida primary-key indeksini, `HasMore`da `languages_is_active_id_idx` orqali `Index Only Scan`ni, nom qidiruvida `languages_name_trgm_idx`ni ishlatdi. Generic prepared plan ham cursor indeksidan foydalandi. Reja filtrlar va ma'lumot taqsimotiga qarab o'zgaradi.

Bu o'lchovlar lokal service va baza chaqiruvlarini qamrab oladi. HTTP, tarmoq, real trafik, aralash yozish amallari, uzoq muddatli yuklama va million parallel foydalanuvchi tekshirilmagan. Production sig'imini real apparat va trafikda qayta o'lchash kerak. Ikki SQL so'rovi orasida yozuvlar o'zgarsa, pagination flaglari keyingi so'rovgacha eskirishi mumkin; bir xil snapshot kafolati berilmaydi.

`/api/languages-active` barcha aktiv yozuvlarni qaytaradi: uning vaqt, xotira va javob hajmi aktiv yozuvlar soni bilan o'sadi. Millionlab aktiv yozuv bo'lgan foydalanishda `/api/languages?is_active=true&limit=100` orqali sahifalash kerak.
