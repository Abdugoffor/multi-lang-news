# Kategoriyalar API

`categories` va `category_translations` uchun bitta `module/category` moduli ishlaydi: handler -> service -> repository. Ushbu loyihada auth moduli mavjud emas; route'lar language moduli kabi autentifikatsiyasiz ulangan.

| Metod | Manzil | Javob |
| --- | --- | --- |
| POST | `/api/categories` | Yaratish, `201` |
| GET | `/api/categories` | Cursor pagination bilan ro'yxat, `200` |
| GET | `/api/categories/:id` | Default va barcha saqlangan tarjimalar, `200` |
| PUT | `/api/categories/:id` | To'liq yangilash, `200` |
| DELETE | `/api/categories/:id` | Kategoriya va uning tarjimalarini o'chirish, bodysiz `204` |

Create va Update uchun bir xil body:

```json
{
  "default": "Elektronika",
  "is_active": true,
  "sort_order": 1,
  "translations": [
    { "language_id": 1, "name": "Elektronika" },
    { "language_id": 2, "name": "Электроника" },
    { "language_id": 3, "name": "Electronics" }
  ]
}
```

`default` majburiy, tilga bog'liq bo'lmagan nom; bazada `categories.default_name`ga yoziladi. `default` va tarjima nomlari chetidagi bo'shliqlar olib tashlanadi, bo'sh qiymat qabul qilinmaydi, uzunligi ko'pi bilan 255 belgi. `sort_order` 0–2147483647 oralig'ida; yuborilmasa `0`. `is_active` yuborilmasa `false`.

Service mavjud `LangService.ListActive` orqali aktiv tillarni oladi. Create va Update'da har bir aktiv til uchun tarjima yuborish majburiy. Tarjimalarda faqat shu tillarning IDlari bo'lishi mumkin, bitta til ikki marta yuborilmaydi. Aktiv tillar bor bo'lsa, `translations` yuborilmasligi yoki `[]` bo'lishi `400` qaytaradi. Aktiv til bo'lmasa, bo'sh tarjimalar bilan faqat default nomni saqlash mumkin.

Aktiv 5 tildan faqat 3 tasi yuborilsa, `400` javobidagi `missing_languages` qolgan barcha tillarning ID va bazadagi nomini qaytaradi. Masalan, 4 va 5 IDli tillar kiritilmagan bo'lsa:

```json
{
  "error": "translations are required for all active languages",
  "missing_languages": [
    { "language_id": 5, "name": "German" },
    { "language_id": 4, "name": "French" }
  ]
}
```

Bu javob Create va Update uchun bir xil. Barcha yetishmagan aktiv tillar bitta javobda, `ListActive`ning `id DESC` tartibida qaytariladi. Service'da nom `TrimSpace` qilingandan keyin bo'sh qolsa ham shu til ro'yxatga qo'shiladi. Xatoda kategoriya va tarjimalar bazaga yozilmaydi.

PUT tarjimalar ro'yxatini to'liq almashtiradi: barcha aktiv tillardagi tarjimalar yuborilishi kerak; ro'yxatga kirmagan eski, endi aktiv bo'lmagan tillardagi tarjimalar o'chiriladi. `translations: []` faqat aktiv til qolmagan bo'lsa qabul qilinadi va barcha eski tarjimalarni olib tashlaydi. Default, kategoriya maydonlari va tarjimalar bitta tranzaksiyada saqlanadi; xatoda hech biri qisman yozilmaydi. Yozish davomida ishlatilgan til qatorlari `FOR SHARE` bilan himoyalanadi.

Show, Create va Update javobi:

```json
{
  "id": 1,
  "default": "Elektronika",
  "is_active": true,
  "sort_order": 1,
  "translations": [
    { "language_id": 1, "name": "Elektronika" },
    { "language_id": 2, "name": "Электроника" },
    { "language_id": 3, "name": "Electronics" }
  ],
  "created_at": "2026-11-12T10:00:00Z",
  "updated_at": "2026-11-12T10:00:00Z"
}
```

Show barcha saqlangan tarjimalarni, jumladan keyin deaktivatsiya qilingan til tarjimasini ham qaytaradi; `language_id ASC` tartibida, pagination yo'q. Tarjima bo'lmasa `translations: []`. Yangi til qo'shilganda unga avtomatik tarjima yaratilmaydi. Create/Update tarjimalarni body tartibida qaytaradi.

List misoli: `GET /api/categories?language_id=2&limit=10&is_active=true`.

| Query | Tavsif |
| --- | --- |
| `language_id` | Musbat til IDsi. Tarjima mavjud bo'lsa shu nom, aks holda default nom |
| `name` | Javobda tanlangan nom bo'yicha katta-kichik harfsiz qidirish |
| `is_active` | Faqat `true` yoki `false`; yuborilmasa barcha kategoriyalar |
| `limit` | 1–100, standart `10` |
| `direction` | `next` yoki `prev`, standart `next` |
| `cursor` | Musbat kategoriya IDsi, `prev` uchun majburiy |

`language_id` yuborilmasa default nom ishlatiladi. Mavjud bo'lmagan til yoki tarjimasi yo'q kategoriya uchun ham default qaytadi. List saqlangan tarjimani til keyin deaktivatsiya qilingan bo'lsa ham ishlatadi; aktivlik talabi Create/Update uchun qo'llanadi.

```json
{
  "data": [
    {
      "id": 1,
      "name": "Электроника",
      "is_active": true,
      "sort_order": 1,
      "created_at": "2026-11-12T10:00:00Z",
      "updated_at": "2026-11-12T10:00:00Z"
    }
  ],
  "next_cursor": null,
  "prev_cursor": null,
  "has_next": false,
  "has_prev": false
}
```

Til modulidagi kabi sahifalar `id DESC` tartibida qaytariladi; `sort_order` saqlanadi va javobga chiqadi, pagination tartibini o'zgartirmaydi. `OFFSET` va umumiy `COUNT` ishlatilmaydi: repository `List` metodi faqat `limit + 1` qatorli slice qaytaradi. Service ortiqcha qatorni ajratadi, `prev` natijasini teskarilaydi, qarama-qarshi yo'nalishni repositorydagi `HasMore` orqali tekshiradi va cursorlarni tayyorlaydi. `HasMore` til, nom va aktivlik filtrlarini saqlaydi. Keyingi so'rovda tegishli cursor bilan birga til va boshqa filterlarni ham yuboring. Ikki SQL so'rovi orasidagi parallel o'zgarishlar uchun bitta snapshot kafolatlanmaydi.

Noto'g'ri body, til IDsi, takroriy yoki aktiv bo'lmagan tarjima tili, ID/query formati uchun `400`; kategoriya topilmasa `404`; FK/unique ziddiyati yoki parallel yozish ziddiyatida `409`. Kutilmagan xato loglanadi va umumiy `500` qaytariladi. Xatolar `{"error":"..."}`, DTO validatsiyasi `{"errors":{...}}`. Body ko'pi bilan 1 MiB, noma'lum maydonlar va ortiqcha JSON qiymatlari qabul qilinmaydi.

`007_categories_default_and_translation_constraints.sql` migratsiyasi `default_name` ustunini qo'shadi. Eski `description` ustuni saqlanadi, ushbu API uni o'zgartirmaydi. Update mavjud tarjimalarni yangilaydi va yangi tillardagi tarjimalarni qo'shadi; `(category_id, language_id)` unique cheklovi yoki `ON CONFLICT`ga tayanmaydi.

Repositoryda CRUD metodlari va language modulidagi kabi pagination uchun `HasMore` bor. Service Create va Update'da avval bitta siklda tarjimalarni `language_id -> name` mapiga yig'adi, nomlarni `TrimSpace` qiladi va takroriy tilni rad etadi. Tarjimalar soni aktiv tillar soniga teng bo'lishi kerak. Keyingi alohida siklda har bir aktiv til uchun mapda tarjima borligi va nomi bo'sh emasligi tekshiriladi. Ichma-ich sikl yo'q. Yetishmagan, ortiqcha, yo'q yoki aktiv bo'lmagan til `400` qaytaradi. Repository tarjimalarni bitta SQL so'rovida yozadi, Go sikli ishlatilmaydi.

Service tegishli CRUD metodini chaqiradi; repository `Create`, `Update` va `Delete` ichida tranzaksiya ochib, bir nechta jadvalga yozishni atomar bajaradi. `Delete` avval shu kategoriyaning tarjimalarini, keyin kategoriyaning o'zini o'chiradi. Bazadagi boshqa FK bog'liqligi o'chirishga to'sqinlik qilsa, butun tranzaksiya, shu jumladan tarjimalarni o'chirish ham bekor qilinadi.
