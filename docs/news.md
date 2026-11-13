# Yangiliklar API

`module/news` handler -> service -> repository qatlamlaridan iborat. Route'lar mavjud category moduli kabi autentifikatsiyasiz ulangan.

| Metod | Manzil | Natija |
| --- | --- | --- |
| POST | `/api/news` | Yaratish, `201` |
| GET | `/api/news` | Cursor pagination bilan ro'yxat, `200` |
| GET | `/api/news/:id` | Yangilik va barcha tarjimalar, `200` |
| GET | `/api/news/:id/view` | Yangilikni Redisdan olish va ko'rish sonini oshirish, `200` |
| PUT | `/api/news/:id` | To'liq yangilash, `200` |
| DELETE | `/api/news/:id` | Yangilik, tarjimalar va kategoriya bog'lanishlarini o'chirish, bodysiz `204` |

Create va Update body misoli (aktiv tillar IDlari `1` va `2` bo'lganda):

```json
{
  "is_main": true,
  "is_active": true,
  "photo": "/uploads/news/photo.jpg",
  "video": "https://example.com/video.mp4",
  "user_id": 1,
  "category_ids": [1, 2],
  "translations": [
    {
      "language_id": 1,
      "title": "Yangi yangilik",
      "description": "Yangilikning to'liq matni."
    },
    {
      "language_id": 2,
      "title": "Latest news",
      "description": "Full news text."
    }
  ]
}
```

`photo` va `video` fayl manzili yoki URLni saqlovchi matn; yuborilmasa bo'sh satr. Fayl yuklash bu APIga kirmaydi. `is_main` va `is_active` yuborilmasa `false`; bir nechta yangilik `is_main: true` bo'lishi mumkin. `user_id` musbat `BIGINT`, body orqali olinadi. Loyihada users/auth moduli va users jadvali yo'qligi sababli foydalanuvchi mavjudligi tekshirilmaydi, users uchun FK qo'yilmagan.

`category_ids` kamida bitta mavjud kategoriya IDsi bilan yuboriladi, IDlar takrorlanmaydi. Bitta yangilik bir nechta kategoriyaga, bitta kategoriya bir nechta yangilikka tegishli bo'lishi mumkin. Kategoriya mavjudligi `CategoryService.Show` orqali tekshiriladi; kategoriya aktivligi uchun qo'shimcha talab yo'q. PUT kategoriya bog'lanishlarini yuborilgan ro'yxatga to'liq almashtiradi.

Service `LangService.ListActive` orqali aktiv tillarni oladi. Har bir aktiv til uchun bitta tarjima majburiy; aktiv bo'lmagan, mavjud bo'lmagan, ortiqcha yoki takroriy til qabul qilinmaydi. `title` majburiy, ko'pi bilan 255 belgi; `description` majburiy `TEXT`. Nom, matn va media manzillari chetidagi bo'shliqlar tozalanadi. Aktiv tillar bo'lmasa `translations: []` yoki translations maydonisiz yaratish/yangilash mumkin.

Yetishmagan tarjimalar uchun `400` javobi, category modulidagi kabi:

```json
{
  "error": "translations are required for all active languages",
  "missing_languages": [
    { "language_id": 2, "name": "English" }
  ]
}
```

`missing_languages` `ListActive`ning `id DESC` tartibida qaytariladi. Service'da bo'shliqlar tozalangandan so'ng title yoki description bo'sh qolsa ham til yetishmagan hisoblanadi. DTO teglari bilan aniqlangan xatolar `{"errors":{...}}` shaklida qaytadi.

PUT barcha aktiv tillar tarjimalarini talab qiladi, ro'yxatda bo'lmagan eski tarjimalar o'chiriladi. Saqlanayotgan tarjima yangilanganda uning `created_at` qiymati saqlanadi va `updated_at` yangilanadi. Yangilik, tarjimalar va kategoriya bog'lanishlari bitta tranzaksiyada yoziladi; xatoda barcha yozishlar bekor qilinadi. Til aktivligi faqat service'da tekshiriladi, repositoryda `JOIN languages` yoki aktivlik filtri yo'q; tekshiruv va yozish orasida aktivlik parallel o'zgarishi mumkin. Til yoki kategoriya o'chirilsa, FK ziddiyati `409` qaytaradi.

Show, Create va Update javobi:

```json
{
  "id": 1,
  "is_main": true,
  "is_active": true,
  "view_count": 0,
  "photo": "/uploads/news/photo.jpg",
  "video": "https://example.com/video.mp4",
  "user_id": 1,
  "category_ids": [1, 2],
  "translations": [
    {
      "news_id": 1,
      "language_id": 1,
      "title": "Yangi yangilik",
      "description": "Yangilikning to'liq matni.",
      "created_at": "2026-11-12T10:00:00Z",
      "updated_at": "2026-11-12T10:00:00Z"
    },
    {
      "news_id": 1,
      "language_id": 2,
      "title": "Latest news",
      "description": "Full news text.",
      "created_at": "2026-11-12T10:00:00Z",
      "updated_at": "2026-11-12T10:00:00Z"
    }
  ],
  "created_at": "2026-11-12T10:00:00Z",
  "updated_at": "2026-11-12T10:00:00Z"
}
```

`category_ids` o'sish tartibida, tarjimalar `language_id ASC` tartibida qaytadi. Show saqlangan barcha tarjimalarni, keyin deaktivatsiya qilingan tillarni ham qaytaradi. Tarjima bo'lmasa `translations: []`. `view_count` bazada `0` bilan boshlanadi, body orqali o'zgartirilmaydi; List/Show uni avtomatik oshirmaydi. IDlar va vaqtlar ham server tomonidan belgilanadi.

`GET /api/news/:id/view` alohida `View` handler va service metodiga ulangan. Yangilikning o'zi har safar mavjud repository `Show` orqali bazadan olinadi. Redisda faqat `test:<DB_NAME>:news:views:<id>` kaliti va uning qiymati sifatida ko'rishlar soni saqlanadi; yangilik JSONi yozilmaydi. `SetNX` kalit yo'q bo'lsa bazadagi `view_count`ni boshlang'ich son qilib saqlaydi, mavjud hisobni almashtirmaydi. Keyin `Incr` hisobni atomar `+1` qiladi. Javob formati Show bilan bir xil, `view_count` Redisdagi yangilangan son bo'ladi. Masalan, bazada `10` bo'lsa ketma-ket javoblar `11`, `12`, `13` qaytaradi. Lua ishlatilmaydi.

View javobi `Cache-Control: no-store` bilan qaytadi. Bir odam qayta ochsa ham hisob oshadi. Yangilik bazada bo'lmasa `404` qaytadi va Redisdagi hisob oshirilmaydi; Redis xatosida umumiy `500` qaytadi. Hisobga TTL qo'yilmagan, u PostgreSQLga yozilmaydi. Redis kaliti o'chirilsa hisob yana bazadagi sondan boshlanadi. Avvalgi `test:<DB_NAME>:news:<id>` JSON saqlagan hash kalitlari yangi API tomonidan ishlatilmaydi.

List misoli: `GET /api/news?language_id=1&category_id=2&is_active=true&limit=10`.

| Query | Tavsif |
| --- | --- |
| `language_id` | Musbat til IDsi; shu tildagi title va description tanlanadi |
| `category_id` | Shu kategoriyaga bog'langan yangiliklar |
| `user_id` | Shu foydalanuvchiga tegishli yangiliklar |
| `title` | Tanlangan title bo'yicha katta-kichik harfsiz qidirish |
| `is_main` | Faqat `true` yoki `false` |
| `is_active` | Faqat `true` yoki `false`; yuborilmasa barcha holatlar |
| `limit` | 1–100, standart `10` |
| `direction` | `next` yoki `prev`, standart `next` |
| `cursor` | Musbat yangilik IDsi; `prev` uchun majburiy |

Til yuborilmasa yoki so'ralgan tarjima mavjud bo'lmasa, saqlangan tarjimalardan eng kichik `language_id`dagi tarjima tanlanadi. Hech qanday tarjima bo'lmasa title va description bo'sh satr. Til keyin deaktivatsiya qilingan bo'lsa ham saqlangan tarjima ko'rsatiladi.

List `data`, `next_cursor`, `prev_cursor`, `has_next`, `has_prev` qaytaradi. Har bir elementda yangilikning barcha asosiy maydonlari, `category_ids`, tanlangan `title` va `description` bor; `translations` List'da qaytarilmaydi. Kategoriyalar bo'yicha filtrlash yangilikni takrorlamaydi. Tartib `id DESC`; `List` repositoryda `limit + 1` yozuv oladi, service cursorlarni tayyorlaydi va `HasMore` qarama-qarshi yo'nalishni bir xil filtrlar bilan tekshiradi. Keyingi so'rovda cursor bilan birga barcha filtrlarni saqlang. List va HasMore orasidagi parallel o'zgarishlar uchun yagona snapshot kafolatlanmaydi.

Noto'g'ri body, query, kategoriya yoki tarjimalar uchun `400`, yangilik topilmasa `404`, FK/unique yoki parallel yozish ziddiyatida `409`. Kutilmagan xato loglanadi, mijozga umumiy `500` qaytadi. Body ko'pi bilan 1 MiB; noma'lum maydonlar va ortiqcha JSON qiymatlari rad etiladi.

`008_news.sql`, `009_news_translations.sql` va `010_news_categories.sql` har bir jadval uchun alohida migratsiyalardir, mavjud migratsiya mexanizmi ularni server ishga tushganda qo'llaydi. FKlar `ON DELETE RESTRICT`: bog'langan kategoriya yoki tilni o'chirishdan oldin yangilik bog'lanishlari olib tashlanadi. News Delete avval bog'lanish va tarjimalarni, so'ng yangilikni o'sha tranzaksiyada o'chiradi. Parallel PUT va DELETE shu yangilik qatori orqali ketma-ket bajariladi.
