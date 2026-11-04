# Autentifikatsiya

`008_create_users.sql` migratsiyasi dastur ishga tushganda mavjud migratsiya mexanizmi orqali bajariladi.

| Endpoint | So'rov | Natija |
| --- | --- | --- |
| `POST /api/auth/register` | `name`, `email`, `password` | HTTP `201`, foydalanuvchining statusi `new` |
| `POST /api/auth/login` | `email`, `password` | HTTP `200`, `access_token`, `token_type`, `expires_at`, `user` |
| `GET /api/auth/me` | `Authorization: Bearer <access_token>` | HTTP `200`, joriy foydalanuvchi |
| `POST /api/auth/logout` | `Authorization: Bearer <access_token>` | HTTP `204`, joriy token bekor qilinadi |

Registratsiya namunasi:

```json
{
  "name": "Ali",
  "email": "ali@example.com",
  "password": "StrongPass123!"
}
```

Login namunasi:

```json
{
  "email": "ali@example.com",
  "password": "StrongPass123!"
}
```

Statuslar: `new`, `active`, `inactive`. Registratsiya doim `new` statusini beradi; so'rovda yuborilgan `status` e'tiborga olinmaydi. Faqat `active` foydalanuvchi login qila oladi. Foydalanuvchilarni boshqarish API'si qo'shilguncha operator PostgreSQL orqali foydalanuvchini aktivlashtiradi:

```sql
UPDATE users SET status = 'active', updated_at = NOW() WHERE id = 1;
```

Hisobni o'chirish uchun `status = 'inactive'` qiymatini qo'llang. Token tekshirilayotganda har bir so'rovda bazadagi joriy status o'qiladi. Shu sababli faol bo'lmagan foydalanuvchi mavjud token bilan ham `/api/auth/me`ga kira olmaydi. Logout orqali faol bo'lmagan foydalanuvchi ham o'z tokenini bekor qila oladi.

Parol kamida 8 belgidan iborat bo'lishi va 72 baytdan oshmasligi kerak. Parollar bcrypt bilan xeshlanadi. Tokenlar 24 soat amal qiladi; PostgreSQL'da faqat ularning SHA-256 xeshlari saqlanadi. Tokenni `Authorization` sarlavhasi orqali yuboring. Auth javoblarida `Cache-Control: no-store` qo'llanadi.

Email manzilining chetidagi bo'shliqlar olib tashlanadi va harflari kichiklashtiriladi. Noto'g'ri so'rov va takroriy email uchun `400` qaytariladi. JSON mavjud handlerlardagi kabi `json.NewDecoder(r.Body).Decode(&req)` orqali o'qiladi; noma'lum maydonlar e'tiborga olinmaydi. Noto'g'ri login ma'lumotlari yoki yaroqsiz/muddati tugagan token uchun `401`; statusi `active` bo'lmagan foydalanuvchi uchun `403` qaytariladi. Kutilmagan xatolarda baza tafsilotlarisiz `500` qaytariladi.

Mavjud category va hudud endpointlarining kirish qoidalari o'zgartirilmagan.

Tekshiruvlar: `go test ./...` va `go vet ./...`. PostgreSQL auth integratsiya testini ham bajarish uchun `AUTH_TEST_DATABASE_URL`ga test bazasining ulanish URL'ini yozing. Integratsiya testi alohida vaqtinchalik sxema yaratadi, auth migratsiyasini shu sxemada bajaradi va test tugagach sxemani olib tashlaydi.
