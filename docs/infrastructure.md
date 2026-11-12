# PgBouncer va Redis

PostgreSQL kompyuterga o'rnatilgan holda qoladi. `compose.yaml` faqat PgBouncer va Redisni ishga tushiradi:

```text
Go -> 127.0.0.1:6432 -> PgBouncer -> host.docker.internal:5432 -> PostgreSQL
Go -> 127.0.0.1:6380 -> Redis:6379
```

## Ishga tushirish

Mavjud `.env`ni saqlang. Yangi o'rnatishda `.env.example`ni `.env`ga nusxalang va PostgreSQL paroli hamda tasodifiy Redis parolini kiriting. Parollarni logga yoki buyruq argumentlariga yozmang.

```powershell
docker compose config --quiet
docker compose up -d --wait
docker compose ps
```

Ikkala xizmat ham `healthy` holatga o'tishi kerak. Xizmatlar `restart: unless-stopped` bilan ishlaydi. Hostdagi portlar faqat `127.0.0.1`ga ochilgan. `6379` bu kompyuterda band bo'lgani uchun Redisning tashqi porti `6380` tanlangan.

| Sozlama | Maqsad | Lokal qiymat |
| --- | --- | --- |
| `DB_HOST`, `DB_PORT` | Go -> PgBouncer | `127.0.0.1`, `6432` |
| `DB_NAME`, `DB_USER`, `DB_PASSWORD` | Mavjud PostgreSQL bazasi va hisob | Mavjud qiymatlar |
| `DB_SSLMODE` | Go -> PgBouncer TLS | Lokal ulanishda `disable` |
| `POSTGRES_HOST`, `POSTGRES_PORT` | PgBouncer -> PostgreSQL | `host.docker.internal`, `5432` |
| `POSTGRES_SSLMODE` | PgBouncer -> PostgreSQL TLS | `prefer` |
| `PGBOUNCER_PORT` | PgBouncerning host porti | `6432` |
| `PGBOUNCER_ADMIN_USERS` | PgBouncer admin konsoliga kiruvchi foydalanuvchi | `postgres` |
| `REDIS_HOST`, `REDIS_PORT` | Go -> Redis | `127.0.0.1`, `6380` |
| `REDIS_PASSWORD` | Redis autentifikatsiyasi | Tasodifiy parol |
| `REDIS_DB` | Redis bazasi | `0` |

`PGBOUNCER_PORT`ni o'zgartirsangiz, `DB_PORT`ni ham moslang. PostgreSQL boshqa kompyuterda bo'lsa, `POSTGRES_HOST`ga o'sha kompyuter IP yoki DNS manzilini yozing. PostgreSQLning `listen_addresses`, `pg_hba.conf` va firewall sozlamalari konteynerdan kelgan ulanishga ruxsat berishi kerak. Konteynerdagi `localhost` host kompyuterini anglatmaydi.

PgBouncer `transaction` rejimida ishlaydi: baza ulanishi tranzaksiya tugagach poolga qaytadi. `max_prepared_statements=200` pgxning protocol prepared statementlarini qo'llaydi. Ko'p buyruqli tranzaksiyalar uchun `statement` rejimini ishlatmang. [PgBouncer sozlamalari](https://www.pgbouncer.org/config).

Go tomonidagi mavjud `pgxpool` saqlangan (`MaxConns=50`, `MinConns=10`). PgBouncerda odatiy pool `20`, zaxira `5`, bir baza uchun yuqori chegara `25`, mijozlar chegarasi `500`. Boshqa dasturlar va PgBouncer nusxalarining ulanishlari ham PostgreSQLning umumiy limitiga kiradi.

Go dasturini yangi kod va `.env` bilan qayta ishga tushiring. `main.go` Redis ulanishini handlerlar yaratilishidan oldin bir marta tayyorlaydi. Mavjud ishga tushish mexanizmi bazaga ulanadi va migratsiyalarni tekshiradi.

Tekshiruv uchun ishga tushirilgan Go fon serveri to'xtatilgan. PgBouncer va Redis Dockerda ishlashda davom etadi. Go serverni terminaldan ishga tushiring; uni `Ctrl+C` bilan to'xtating. `8082` portida bir vaqtning o'zida bitta server nusxasi ishlaydi:

```powershell
go run .
```

## Faol tillar keshi

`GET /api/languages-active`, news va category tekshiruvlari bitta `LangService.ListActive` metodidan foydalanadi:

1. Avval Redis o'qiladi. Kesh bo'lsa, SQL bajarilmaydi.
2. Kesh bo'lmasa, mavjud `LangRepository.ListActive` metodining o'zida PostgreSQLdan o'qiladi va natija 60 soniyaga Redisga yoziladi.
3. Muvaffaqiyatli til `Create`, `Update`, `Delete` amallaridan keyin service eski keshni bekor qiladi va o'zining `ListActive(ctx)` metodini chaqiradi. Shu metod faol tillarni bazadan olib Redisga saqlaydi; o'qish va keshga yozish mantiqi takrorlanmaydi. Faollik o'zgarmagan yangilanishlar ham keshdagi nom va vaqtlarni yangilaydi.

Kalitlar `test:<DB_NAME>:{languages}:active` va `test:<DB_NAME>:{languages}:version`. Natija avvalgidek `id DESC` tartibidagi JSON massiv; bo'sh ro'yxat ham `[]` sifatida keshlanadi. Oddiy `/api/languages` pagination ro'yxati keshlanmaydi.

Redis va PostgreSQLdan faol tillarni olish mavjud `lang_repo.go`dagi `ListActive(ctx)` metodida bajariladi. Service o'zgarishdan keyin repositoryning `InvalidateActive` metodini chaqirib, so'ng `ListActive(ctx)` orqali keshni yangilaydi. Alohida cache repository yo'q. Versiyani tekshirish va yozish Redis Lua orqali atomar bajariladi: til o'zgarishidan oldin boshlangan o'qish yangi kesh ustiga eski ro'yxatni yozmaydi. Bu himoya bir Redisni ishlatadigan bir nechta Go nusxalariga ham taalluqli.

Redis ishlamasa, o'qishlar PostgreSQLga o'tadi; muvaffaqiyatli DB yozuvi kesh xatosi sababli HTTP xatoga aylantirilmaydi. Xato logga yoziladi. Kesh yangilash request bekor qilinganidan keyin ham ko'pi bilan 3 soniya davom etadi. DB va Redis orasida umumiy tranzaksiya yo'q: Redis uzilishi yoki jarayon to'xtashi vaqtida yangilash bajarilmasa, avvalgi kesh TTL tugaguncha qolishi mumkin. Bazaga API tashqarisidan o'zgartirish kiritilsa ham shu TTL amal qiladi.

Redis qayta ishga tushganda kesh bo'shaydi va keyingi o'qishda tiklanadi; kesh uchun diskka yozish o'chirilgan. Versiya kalitini faol keshdan alohida qo'lda o'chirmang.

## Boshqarish

PgBouncer admin konsoli shu loyihadagi `DB_USER` va `DB_PASSWORD`dan foydalanadi. Hozirgi lokal hisob `postgres`; parol `.env`dagi `DB_PASSWORD`. Admin bazasi `pgbouncer`, loyiha bazasi esa `golang`. Admin huquqi `PGBOUNCER_ADMIN_USERS=postgres` orqali berilgan. Bu PostgreSQLga yangi foydalanuvchi yaratmaydi yoki uning parolini almashtirmaydi.

```powershell
docker compose exec pgbouncer psql -h 127.0.0.1 -p 6432 -U postgres -d pgbouncer
```

Parol so'ralganda `DB_PASSWORD` qiymatini kiriting. Keyin psql ichida:

```sql
SHOW POOLS;
SHOW STATS;
SHOW DATABASES;
```

`.env`dagi paroldan avtomatik foydalanish uchun PowerShellda:

```powershell
docker compose exec pgbouncer sh
```

Ochilgan konteyner shellida:

```sh
export PGPASSWORD="$DB_PASSWORD"
psql -h 127.0.0.1 -p 6432 -U "$DB_USER" -d pgbouncer
```

Postmandagi `http://127.0.0.1:8082/api/news/1` HTTP APIga tegishli. PgBouncer hisobi HTTP Basic Auth uchun ishlatilmaydi. `ECONNREFUSED ...:8082` bo'lsa, o'sha vaqtda Go server ulanishni qabul qilmagan; PgBouncer noto'g'ri paroli boshqa portdagi autentifikatsiya xatosini beradi. Yuklama boshida 1000 yangi ulanishni birdan ochish Windows accept navbatini to'ldirishi mumkin; `.env`dagi DB parolini yoki PgBouncer client limitini almashtirish bu HTTP navbatini oshirmaydi.

```powershell
docker compose logs --tail 50 pgbouncer redis
docker compose up -d --force-recreate
docker compose stop
docker compose start --wait
```

Parol yoki ulanish sozlamasi o'zgarsa, konteynerlarni `up -d --force-recreate` bilan qayta yarating va Go dasturini qayta ishga tushiring. `docker compose config`ni `--quiet`siz chiqarish parollarni ko'rsatishi mumkin.

## Tekshiruv

2026-11-12 kuni ikkala konteyner `healthy` bo'ldi. Haqiqiy Redis bilan vaqtinchalik tekshiruvlarda keshning bo'sh natijasi, keshdan o'qish, create/update/delete orqali yangilash, nom/faollik o'zgarishi, muvaffaqiyatsiz yozuvda keshni saqlash, eskirgan parallel yozuvni rad etish va Redis mavjud bo'lmaganda bazaga qaytish o'tdi. CRUD tekshiruvlari xotiradagi izolyatsiyalangan repository bilan bajarildi; mavjud PostgreSQL yozuvlari o'zgartirilmadi.

Haqiqiy PostgreSQLdan PgBouncer orqali faol tillar o'qildi; 8 parallel mijozda jami 240 parametrli pgx so'rovi muvaffaqiyatli bajarildi. `go build ./...` va `go vet ./...` o'tdi. Vaqtinchalik test fayli tekshiruvdan keyin olib tashlandi.

Yangilangan HTTP serverda `/api/languages-active` ikki marta `200` qaytardi; 5 ta faol til Redisdagi `test:golang:{languages}:active` kalitiga yozildi va 60 soniyalik TTL tasdiqlandi. Ishga tushishda yangi migratsiya qo'llanilmadi (`Migrations: nothing new`).

Redis mantiqi mavjud `lang_repo.go`ga birlashtirilgach, haqiqiy repository vaqtinchalik izolyatsiyalangan PostgreSQL va haqiqiy Redis bilan qayta tekshirildi: bo'sh ro'yxat, DBsiz cache hit, create/update/delete yangilashi, faol bo'lmagan til, muvaffaqiyatsiz o'chirish, buzilgan kesh JSONini tiklash, TTL va Redis mavjud bo'lmaganda SQLga qaytish o'tdi. Cache miss SQL so'rovi vaqtincha jadval locki bilan to'xtatilib, kesh versiyasi yangilangach eski o'qish yangi keshni almashtirmasligi ham tasdiqlandi. Vaqtinchalik konteyner va tekshiruv dasturi olib tashlandi.
