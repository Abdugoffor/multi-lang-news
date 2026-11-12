# Loyiha arxitekturasi va kod yozish qoidalari

Ushbu qoidalar butun loyihaga, mavjud va yangi modullarga tegishli. Kod yozishdan oldin ushbu faylni, o'zgartiriladigan modulni va ishlatiladigan mavjud funksiyalarni o'qing. Arxitektura, nomlash, qavslar va mantiqiy bloklar uchun amaldagi namunalar — `module/news` va `module/category`. Foydalanuvchi ushbu modullarda qo'lda joylashtirgan mustaqil `{ ... }` bloklar keyingi o'zgarishlarda ham saqlanadi.

## 1. Barcha amallar uchun majburiy arxitektura

Har bir modul quyidagi yo'nalishda ishlaydi:

```text
HTTP request -> Handler -> Service -> Repository -> Ma'lumot saqlash joyi
HTTP response <- Handler <- Service <- Repository <- Natija
```

- CRUD, login, logout, ro'yxatdan o'tish, qidirish, tasdiqlash, holat almashtirish, forma olish va boshqa barcha amallar shu qatlam chegaralariga amal qiladi.
- Handler faqat HTTP qismini bajaradi va o'z service metodini chaqiradi.
- Service biznes qoidalarini tekshiradi, jarayonni boshqaradi va repository orqali ma'lumotlar bilan ishlaydi.
- Repository SQL va fayl kabi saqlash joylariga oid o'qish/yozishni bajaradi.
- Handlerdan repositoryga yoki bazaga to'g'ridan-to'g'ri o'tish taqiqlanadi.
- Serviceda SQL, `QueryRow`, `Query`, `Exec` yoki biznes ma'lumotlarini faylga bevosita yozish bo'lmasin.
- Repository handler yoki serviceni chaqirmaydi, biznes jarayonini boshqarmaydi.
- Yangi amal uchun mavjud repository metodi mos kelsa, qayta ishlating. Ma'lumot o'qish/yozish talab qilinmasa, sun'iy repository metodi yaratmang; amal handlerdan service orqali bajariladi.
- Arxitekturani chetlab o'tadigan qisqa yo'l, boshqa handlerni chaqirish yoki biznes mantiqini helperga ko'chirish bilan vazifani hal qilmang.
- Vazifaga aloqasi bo'lmagan modullar, API xulqi va formatni o'zgartirmang.

## 2. Modulning fayl tuzilishi

Go modul nomi `auth-login`, `go.mod`dagi Go versiyasi `1.24.4`. Loyiha `net/http`, `httprouter`, `pgx/v5`, `pgxpool` va `validator/v10`dan foydalanadi.

Har bir modul mavjud tuzilishga mos yoziladi:

```text
module/<module>/
  cmd.go
  dto/
    req_dto.go
    res_dto.go
  handler/
    <module>_handler.go
  service/
    <module>_service.go
  repository/
    <module>_repository.go
```

- `cmd.go`dagi `Cmd(router, group, db)` modul handlerini ulaydi.
- Route'lar handler konstruktorida tegishli modulning route blokida ro'yxatdan o'tkaziladi.
- `main.go` yangi modulning `Cmd` funksiyasini `/api` guruhi bilan chaqiradi.
- Request va filter turlari `dto/req_dto.go`, response turlari `dto/res_dto.go`da bo'ladi.
- Alohida saqlash vazifasi kerak bo'lsa, modulning `repository/` papkasida qo'shimcha repository bo'lishi mumkin. Masalan, application modulidagi `VerificationRepository`.
- Bir nechta modulga kerak bo'ladigan, biznesga bog'lanmagan yordamchilar `helper/`da saqlanadi.
- Sozlamalar va ulanishlar `config/`, raqamlangan SQL migratsiyalar `config/migrations/`da bo'ladi.

## 3. Qatlamlarning aniq mas'uliyati

### Handler

Handler quyidagi tartibda ishlaydi:

1. Body, path, query yoki headerdan ma'lumot oladi.
2. Transport validatsiyasini bajaradi: JSON, ID, query formati va DTO teglarini tekshiradi.
3. `r.Context()` va tayyorlangan DTOni service metodiga uzatadi.
4. Service xatosini HTTP javobiga aylantiradi yoki natijani qaytaradi.

`helper.DecodeJSON`, `helper.Validate`, `helper.ParseID`, `helper.JSON` va mavjud modul yordamchilarini qayta ishlating. Matnlarni `strings.TrimSpace` bilan tozalash handlerda mumkin. Boshqa chaqiruvchilar uchun ham bajarilishi kerak bo'lgan biznes normalizatsiyasi va tekshiruvlari serviceda qoladi.

Handlerda SQL, repository maydoni, bazaga murojaat, fayl saqlash yoki biznes jarayonini boshqarish bo'lmasin. Handler konstruktoridagi `db` faqat bog'liqliklarni ulash uchun ishlatiladi; request metodlari bazadan foydalanmaydi.

Query filterlarini `List` handlerining o'zida har bir maydon uchun ketma-ket tekshiring. Alohida `parseFilter`, struct/pointerlar orqali umumiy parse sikli yoki oddiy filterlar uchun ortiqcha abstraksiya yaratmang.

### Service

- Sana munosabati, tegishlilik, faollik, parent, sikl, tasdiqlash holati va boshqa biznes shartlarini tekshiradi.
- Repository va boshqa modullarning mavjud servicelarini chaqirib, amal ketma-ketligini boshqaradi.
- Natijani DTOga tayyorlaydi va xatolarni chaqiruvchiga qaytaradi.
- `context.Context`ni repository va boshqa servicelarga uzatadi.
- `http.ResponseWriter`, `*http.Request`, HTTP status yozish yoki HTTP javobi uchun JSON chiqarishdan foydalanmaydi.
- Tayyor SQL natijasini qaytarish yetarli bo'lsa, uni keraksiz sikl, map yoki qayta JSON yig'ish bilan murakkablashtirmaydi.

Oddiy delegatsiyada to'g'ridan-to'g'ri `return service.repository.Show(ctx, id)` yozish mumkin. Bu service qatlamini olib tashlashga ruxsat bermaydi.

### Repository

- Parametrli SQL, `QueryRow`, `Query`, `Exec`, `Scan` va saqlash amallarini bajaradi.
- Fayl repositorysi faylni o'qish, yozish va almashtirishni bajaradi; kodning amal qilish vaqti yoki urinishlar soni kabi biznes qoidalari serviceda tekshiriladi.
- Natija va xatoni qaytaradi; HTTP status yoki JSON javobini yozmaydi.
- JWT tekshirmaydi, request parse qilmaydi va service chaqirmaydi.
- Boshqa repositorylarni chaqirib biznes jarayonini zanjirlamaydi; ketma-ketlikni service boshqaradi.
- `Scan` har bir metod ichida ochiq yoziladi. Alohida scan funksiyasi yaratilmaydi.

## 4. Nomlash va bog'liqliklar

- Package va import aliaslari: `tournament_dto`, `tournament_handler`, `tournament_service`, `tournament_repository` kabi `<module>_<layer>` uslubida.
- Handler yopiq struct: `tournamentHandler`.
- Service eksport qilingan konkret struct: `TournamentService`.
- Repository eksport qilingan interface: `TournamentRepository`; implementatsiya yopiq struct: `tournamentRepository`.
- Receiver nomlari qatlamga mos: `handler`, `service`, `repository`.
- Handler o'z serviceni konkret pointer orqali saqlaydi: `*tournament_service.TournamentService`.
- Service repositoryni interface orqali, boshqa serviceni konkret struct pointeri orqali saqlaydi. Service uchun interface yaratmang.
- PostgreSQL repositorysi `db *pgxpool.Pool`ni saqlaydi va mavjud uslubda `repository.db` orqali ishlaydi.
- Bog'liqliklar konstruktorda bir marta ulanadi; har bir request ichida yangi service yoki repository yaratilmaydi.
- Konstruktor uslubi: `NewTournamentHandler(router, group, db)`, `NewTournamentService(db) *TournamentService`, `NewTournamentRepository(db) TournamentRepository`.
- CRUD nomlari: `Create`, `List`, `Show`, `Update`, `Delete`. Boshqa amallarga `Login`, `Logout`, `VerifyEmail`, `Form`, `WithChildren` kabi maqsadli nom bering.
- Mavjud role, status va direction konstantalarini qayta ishlating.

## 5. Qavslarni ochish, yopish va bloklarga olish

Qavslar va mantiqiy bloklar uslubiga har bir o'zgarishda amal qiling. Foydalanuvchi tahrirlagan amaldagi asosiy namunalar:

- `module/news/handler/news_handler.go`
- `module/news/service/news_service.go`
- `module/news/repository/news_repository.go`

Qo'shimcha namunalar — `module/category` va `module/language`ning tegishli qatlamlari. Loyihada mavjud bo'lmagan `module/tournament` fayllarini qavslar uslubi uchun manba sifatida qidirmang. Quyidagi qoidalar yangi kodga ham, mavjud kodni o'zgartirishga ham tegishli; vazifaga aloqasiz kodni bir xil ko'rinishga keltirish uchun qayta yozmang.

### Umumiy qavs qoidalari

- Funksiya, metod, `if`, `else`, `for`, `switch` va structning ochiluvchi `{` qavsi o'sha ifoda bilan bir qatorda yoziladi.
- Mustaqil mantiqiy blokning `{` qavsi alohida qatorda ochiladi.
- Har bir ochilgan `{` o'zining `}` qavsi bilan yopiladi. Yopiluvchi qavs uni ochgan konstruksiya bilan bir xil chekinishda turadi.
- `else` oldingi blokning yopiluvchi qavsi bilan bir qatorda yoziladi: `} else {`.
- Tab bilan indentatsiya qiling. O'zgargan Go fayllariga `gofmt` ishlating.
- Mantiqiy bosqichlarni bo'sh qator bilan ajrating. Xato aniqlanganda erta `return` qiling.
- Mavjud mustaqil bloklarni faqat kodni qisqartirish uchun olib tashlamang.
- Har bir satr yoki har bir `if` uchun sun'iy blok yaratmang. Requestni tayyorlash, chaqiruv natijasini tekshirish va route'larni guruhlash kabi mantiqiy bosqichlarni blokka oling.
- Deklaratsiya yoki chaqiruvdan keyingi mustaqil blok darhol keyingi satrda ochiladi; ular orasiga bo'sh qator qo'ymang. Blok yopilgach, keyingi mantiqiy bosqichdan oldin bo'sh qator qoldiring.
- Chaqiruv natijasi keyin ishlatilsa, uni blokdan oldin e'lon qiling. Blok natijani va xatoni tekshirish uchun xizmat qiladi; natija blok ichida ko'rinmay qolmasin.
- Foydalanuvchi qo'shgan mustaqil bloklarni "ortiqcha qavs" deb olib tashlamang yoki `if err := ...; err != nil` ko'rinishiga qisqartirmang. `gofmt` indentatsiyani to'g'rilaydi, ushbu bloklarni qo'lda olib tashlash uchun sabab bo'lmaydi.

### Handlerda request va service natijasi

Request deklaratsiyasidan keyin parse qilishni, service chaqiruvidan keyin xatoni tekshirishni alohida blokka oling:

```go
func (handler *newsHandler) Create(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var req news_dto.Create
	{
		if !parseBody(w, r, &req) {
			return
		}
	}

	res, err := handler.service.Create(r.Context(), &req)
	{
		if err != nil {
			writeServiceError(w, err)
			return
		}
	}

	helper.JSON(w, http.StatusCreated, res)
}
```

Path yoki query qiymatini parse qilganda ham natijani tekshirish shu uslubda yoziladi:

```go
id, ok := helper.ParseID(w, params)
{
	if !ok {
		return
	}
}
```

### Serviceda chaqiruv natijasini tekshirish

Repository yoki boshqa service chaqiruvidan keyingi tekshiruvni shu uslubda yozing. Bu natija `:=` bilan e'lon qilinganda ham, oldindan mavjud o'zgaruvchiga `=` bilan yozilganda ham amal qiladi:

```go
rows, err := service.repository.List(ctx, filter)
{
	if err != nil {
		return nil, err
	}
}
```

Oddiy biznes sharti uchun bevosita `if` yetarli:

```go
if len(translations) != len(languages) {
	return nil, ErrInvalidTranslation
}

return service.repository.Create(ctx, req)
```

Natija tashqarida kerak bo'lsa, o'zgaruvchini tekshiruv blokidan oldin e'lon qiling. Blok ichida `:=` bilan tashqi `err` yoki natijani tasodifan soyada qoldirmang.

Faqat xato qaytaradigan ichki tekshiruvlar va validatsiyalar uchun mavjud ixcham `if` uslubi ham saqlanadi. Masalan, `NewsService.Create` va `Update`dagi `prepare` chaqiruvi:

```go
if err := service.prepare(ctx, req); err != nil {
	return nil, err
}

res, err := service.repository.Create(ctx, req)
{
	if err != nil {
		return nil, serviceError(err)
	}
}
```

Bir chaqiruv xatosi uchun bir nechta tekshiruv kerak bo'lsa, ularning barchasi o'sha tekshiruv blokida bo'ladi. Masalan, `service.category.Show`dan keyin avval `errors.Is(err, pgx.ErrNoRows)`, so'ng qolgan `err != nil` tekshiriladi.

### Repositoryda SQL va xato tekshiruvi

Repositoryda ham mustaqil tekshiruv bloklari ishlatiladi. "SQLdan keyin alohida blok bo'lmaydi" degan umumiy cheklov yo'q. Qisqa chaqiruv va uning natijasini tekshirish uchun quyidagi uslubdan foydalaning; mavjud bloklarni saqlang:

```go
rows, err := repository.db.Query(ctx, query, args...)
{
	if err != nil {
		return nil, err
	}
}

defer rows.Close()
```

Shu uslub `HasMore`dagi qisqa `QueryRow(...).Scan(...)` va `Delete`dagi `Exec` natijalariga ham tegishli:

```go
err := repository.db.QueryRow(ctx, query, args...).Scan(&exists)
{
	if err != nil {
		return false, err
	}
}
```

```go
result, err := tx.Exec(ctx, "DELETE FROM news WHERE id = $1", id)
{
	if err != nil {
		return err
	}
}

if result.RowsAffected() == 0 {
	return pgx.ErrNoRows
}
```

`json.Marshal` va tranzaksiyani ochishdan keyingi xato tekshiruvlari ham alohida blokda yoziladi:

```go
translations, err := json.Marshal(req.Translations)
{
	if err != nil {
		return nil, err
	}
}

tx, err := repository.db.Begin(ctx)
{
	if err != nil {
		return nil, err
	}
}

defer tx.Rollback(context.WithoutCancel(ctx))
```

Ko'p qatorli SQL va `Scan`dan keyin amaldagi `Show`, `Create`, `Update` metodlaridagi kabi bevosita `if err != nil` yozish mumkin. Har bir SQL yoki har bir `Scan`ni majburan qo'shimcha blokka o'ramang. `Scan` maydonlarini metod ichida aniq ko'rsating:

```go
var res news_dto.Response

err := repository.db.QueryRow(ctx, `
	SELECT id, is_main, is_active, view_count, photo, video, user_id, created_at, updated_at
	FROM news WHERE id = $1
`, id).Scan(
	&res.ID,
	&res.IsMain,
	&res.IsActive,
	&res.ViewCount,
	&res.Photo,
	&res.Video,
	&res.UserID,
	&res.CreatedAt,
	&res.UpdatedAt,
)

if err != nil {
	return nil, err
}

return &res, nil
```

`rows.Err()` va `tx.Commit(ctx)` kabi faqat xato tekshiriladigan chaqiruvlarning mavjud ixcham shakli saqlanadi:

```go
if err := rows.Err(); err != nil {
	return nil, err
}

if err := tx.Commit(ctx); err != nil {
	return nil, err
}
```

Bir metoddagi barcha `if err != nil`larni mexanik ravishda bir xil ko'rinishga o'tkazmang. O'zgartirilayotgan mantiqiy bosqichning mavjud uslubi va yuqoridagi qisqa/ko'p qatorli chaqiruv namunalariga amal qiling.

### Route'larni blokka olish

Umumiy prefiksdan keyin modul route'larini bitta mantiqiy blokda yozing:

```go
routes := group + "/news"
{
	router.POST(routes, handler.Create)
	router.GET(routes, handler.List)
	router.GET(routes+"/:id", handler.Show)
	router.PUT(routes+"/:id", handler.Update)
	router.DELETE(routes+"/:id", handler.Delete)
}
```

## 6. Mavjud funksiyalarni qayta ishlatish

Yangi funksiya, SQL yoki helper yozishdan oldin:

1. `rg` bilan butun `module/` va `helper/` ichidan mos imkoniyatni qidiring.
2. Topilgan funksiyaning imzosi, filterlari, biznes tekshiruvlari, xatolari va yon ta'sirlarini o'qing.
3. Semantikasi mos bo'lsa, mavjud funksiyani chaqiring. Mos bo'lmasa, APIga zarar bermaydigan kengaytirish yoki alohida metod zaruratini aniqlang.
4. Yangi repository metodi zarur bo'lsa, interface va implementatsiyani birga yangilang.

Biznes tekshiruvlari bor amal uchun boshqa modulning mavjud service metodini ishlating. Faqat ma'lumot olish kerak bo'lsa va biznes tekshiruvlari chetlab o'tilmasa, service boshqa modulning repository interface'idan foydalanishi mumkin. Aylanma import yaratmang.

Asosiy qayta ishlatiladigan imkoniyatlar:

| Joy | Imkoniyat |
| --- | --- |
| `helper/helper.go` | `LoadEnv`, `ENV`, `JSON`, `DecodeJSON`, `Validate`. |
| `helper/http.go` | Route'dagi musbat ID uchun `ParseID`. |
| `helper/auth.go` | `HashPassword`, `CheckPassword`, `NewToken`, `ParseToken`, `HashToken`. |
| `module/auth/service` | `SeedSuperadmin`, `Login`, `Authenticate`, `Logout`. |
| `module/auth/handler` | Route himoyasi uchun `RequireAuth(service, next, roles...)`. |
| `module/tournament/service` | `Create`, `List`, `Show`, `Update`, `Delete`. |
| `module/tournament/repository` | CRUD, cursor pagination uchun `List` va `HasMore`. |
| `module/category/service` | CRUD va kategoriya daraxti uchun `WithChildren`. |
| `module/page/service` | CRUD, `Form`, turnir/kategoriya bog'liqligi va faolligi uchun `ValidateCategory`. |
| `module/field/service` | CRUD va maydonning biznes validatsiyasi. |
| `module/application/repository` | Arizani saqlash va tasdiqlash holati; kod fayli uchun `VerificationRepository`. |

Kichik harf bilan boshlangan funksiya faqat o'z package'ida ishlaydi. Modulga xos `parseBody` va `writeServiceError`ni boshqa moduldan chaqirib bo'lmaydi. Umumiy qism zarur bo'lgandagina mos umumiy joyga ajratiladi.

## 7. Route, HTTP va xatolar

- Alohida API alohida handler va service metodiga ega bo'ladi. Route tegishli modulning route blokida o'z handleriga to'g'ridan-to'g'ri ulanadi.
- Boshqa handler ichida path nomini tekshirib yo'naltirmang.
- `httprouter` yo'llari to'qnashsa, mavjud CRUD manzillarini saqlang va to'qnashmaydigan alohida manzil tanlang. Yangi manzilni hujjatda va yakuniy javobda ayting.
- Himoyalangan route uchun mavjud `auth_handler.RequireAuth`ni ishlating. Token va sessiya tekshiruvlarini boshqa modulda takrorlamang; `AuthService.Authenticate`ni qayta ishlating.
- Mavjud auth qoidalarini saqlang: tournament/category/page/field CRUD yozish amallari admin/superadmin, o'qish amallari autentifikatsiya bilan; application yaratish va email tasdiqlash route'lari ommaviy.
- JSON uchun `helper.JSON`, body uchun `helper.DecodeJSON`, DTO uchun `helper.Validate`ni ishlating. `DecodeJSON`ning hajm, noma'lum maydon va ortiqcha JSON qiymati tekshiruvlarini chetlab o'tmang.
- Xato shakli `{"error":"..."}`, validatsiya xatolari `{"errors":{...}}`.
- Service xatolari handlerdagi modulga xos `writeServiceError`da `errors.Is` yoki `errors.As` bilan HTTP statusga aylantiriladi.
- Mavjud xaritalashni saqlang: yaratilganda `201`, o'qish/yangilashda `200`, o'chirish/logoutda bodysiz `204`; noto'g'ri so'rov `400`, topilmagan yozuv `404`, bog'liqlik yoki holat ziddiyati `409`. Authning `401`/`403`, tasdiqlash urinishlarining `429` qoidalari saqlanadi.
- Kutilmagan xato log qilinadi, mijozga umumiy `500` javobi qaytariladi; ichki SQL yoki fayl xatosi chiqarilmaydi.

## 8. SQL, saqlash va migratsiyalar

- SQL faqat repositoryda yoziladi. Foydalanuvchi qiymatlarini `$1`, `$2` kabi parametrlar bilan uzating.
- Dinamik SQL operatori va tartibi faqat koddagi ruxsat etilgan qiymatlardan tanlanadi. Foydalanuvchi matnini SQLga qo'shmang.
- `Query`dan keyin `defer rows.Close()`, iteratsiyadan keyin `rows.Err()` tekshiruvini yozing.
- `Exec`da yozuv topilganini tekshirish kerak bo'lsa, `RowsAffected()`dan foydalaning.
- Oddiy CRUD uchun ortiqcha `queryer`, `queries` yoki tranzaksiya abstraksiyalarini kiritmang. Mavjud category va tournament repositorylari bevosita `repository.db` bilan ishlaydi.
- Bir nechta yozuvni atomar o'zgartirish zarur bo'lsa, jarayonni service boshqaradi, SQL repositoryda qoladi va tegishli amallar bitta tranzaksiyada bajariladi. Tranzaksiya kiritilsa, interface va implementatsiya birga yangilanadi.
- Baza va faylga yozish avtomatik ravishda bitta tranzaksiya bo'lmaydi. Mavjud kompensatsiya va parallel so'rovlarni himoyalash mexanizmlarini saqlang; atomarlik bo'lmasa, bor deb hisoblamang.
- Schema o'zgarishini `config/migrations/`dagi keyingi raqamli SQL fayliga yozing. Mavjud migratsiya mexanizmini qayta ishlating.
- Har bir yangi jadval uchun alohida migratsiya yozing.
- Foreign key orqali bog'liq yozuvlarni avtomatik o'chirmang. Avval ular alohida o'chirilishi yoki ko'chirilishi kerak.

## 9. Mavjud modullarda saqlanadigan xulq

Bu bo'lim mavjud imkoniyatlarni o'zgartirishdan oldin tekshirish uchun qisqa xarita. Vazifa talab qilmasa, ushbu xulqni o'zgartirmang.

- `auth`: login, sessiya va token tekshiruvlari, logout `AuthService` orqali bajariladi.
- `tournament`: `/api/tournaments`; cursor pagination repositorydagi `List`/`HasMore` va service orqali ishlaydi. Repository `Limit+1` yozuv oladi; service ortiqcha yozuvni ajratadi, `prev` natijasini teskari aylantiradi va qarama-qarshi yo'nalishni `HasMore` bilan tekshiradi. Handlerdagi standart limit `10`, servicedagi fallback `20`.
- `category`: `/api/categories`; parent shu turnirga tegishli bo'ladi va sikl yaratmasligi kerak. Parent tekshiruvlari serviceda `Show` orqali bajariladi; mavjud tekshiruv va yozish bitta tranzaksiya emas, parallel parent o'zgarishlarida sikldan himoya kafolatlanmaydi. `parent_id` yuborilmasa yoki `0` bo'lsa, ro'yxatda bosh kategoriyalar qaytadi.
- Kategoriya daraxti: `GET /api/categories_with_childr?tournament_id=1` bevosita `WithChildren` handleriga ulangan. Service turnirni tekshiradi, repository va `category_children_json` SQL funksiyasi daraxt JSONini yig'adi. Serviceda daraxt qayta yig'ilmaydi; bolalar bo'lmasa `children: []`.
- `page`: `/api/pages`; `GET /api/pages-form?tournament_id=1&category_id=2` bevosita `Form` handleriga ulangan. `PageService.ValidateCategory` turnir/kategoriya munosabatini va kerak bo'lsa faollikni tekshiradi.
- `field`: `/api/fields`; `FieldService` sahifani `PageService.Show` orqali tekshiradi. `options` — `[]string`, `validation` — `field_dto.Validation`; biznes validatsiyasi serviceda.
- Page/Field ro'yxatlarida pagination yo'q. Forma faqat tanlangan kategoriyaning faol sahifa va maydonlarini `sort ASC, id ASC` tartibida qaytaradi; ota kategoriyadan meros olinmaydi. Sahifa `sort >= 3`, maydon `sort >= 1`; tartiblar kategoriya/sahifa ichida takrorlanmaydi. Forma DTOlarida `sort` qaytarilmaydi.
- `application`: `POST /api/applications` va `POST /api/applications/verify-email`; biznes jarayoni `ApplicationService`da, baza va kod fayli amallari repositorylarda. `PageService.ValidateCategory` va `PageService.Form` qayta ishlatiladi.
- Tasdiqlash kodi 6 xonali, 10 daqiqa amal qiladi; 5 xato urinishdan keyin `429`. Kodlar `.local/application_codes.json`da saqlanadi, yo'l `APPLICATION_CODES_FILE` orqali o'zgaradi. Hozir kod logga chiqariladi, email yuborilmaydi. Qayta tasdiqlash `409`.
- Bog'liq kategoriya, sahifa, maydon yoki arizalar bor yozuvlarni o'chirish/ko'chirishdagi mavjud FK cheklovlari va `409` javoblarini saqlang.
- Mavjud API misollari: `docs/page-form.md`, `docs/applications.md`. Tuzilish yoki qayta ishlatiladigan API o'zgarsa, tegishli hujjatlarni ham yangilang.

## 10. Ishni yakunlash va tekshirish

1. Har bir yangi amal `handler -> service -> repository` chegaralariga mosligini tekshiring. Ma'lumot saqlash kerak bo'lmagan amalda bo'sh repository yaratmang.
2. O'zgargan route, handler, service, repository interface/implementatsiyasi, DTO va migratsiyalar o'zaro mosligini tekshiring.
3. Qavslarning ochilishi/yopilishi, mustaqil bloklar, o'zgaruvchilar ko'rinish doirasi va erta `return`larni ko'rib chiqing. Handler/service bilan birga repositorydagi qisqa chaqiruvlarning tekshiruv bloklari ham saqlanganini tekshiring; faqat kodni qisqartirish uchun foydalanuvchi qo'ygan bloklarni olib tashlamang.
4. O'zgargan Go fayllariga `gofmt -w <fayllar>` ishlating.
5. Go kodi o'zgarsa, `go build ./...` va `go vet ./...`ni bajaring. Zarur bo'lsa amaldagi xulqni ham tekshiring. Faqat hujjat o'zgarsa, Go tekshiruvlarini qayta ishga tushirish shart emas.
6. Loyihada test fayllarini qoldirmang. Odatda yangi test faylisiz tekshiring; vaqtinchalik test zarur bo'lsa, tekshiruvdan so'ng o'zingiz yaratgan faylni olib tashlang.
7. Loyihada `.lock` yoki `*.lock` fayllarini yaratmang va qoldirmang.
8. `go run .`ni kompilyatsiya tekshiruvi sifatida ishlatmang: u bazaga ulanadi, migratsiyalar va seederni ishga tushiradi.
9. Tekshiruv bajarilmasa yoki o'tmasa, sababini aniq ayting. Bajarilmagan yoki o'tmagan tekshiruvni muvaffaqiyatli deb yozmang.
10. Foydalanuvchining mavjud o'zgarishlarini saqlang; vazifaga aloqasiz kod va hujjatlarni o'chirmang yoki qayta yozmang.
