# Docker Compose: PostgreSQL, PgBouncer va Redis

```text
Terminaldagi Go -> 127.0.0.1:6432 -> PgBouncer -> postgres:5432
Terminaldagi Go -> 127.0.0.1:6380 -> Redis:6379
```

## Ishga tushirish

Mavjud `.env` va `.env.example` o'zgartirilmaydi. Compose baza nomi, foydalanuvchi, parol, timezone va Redis parolini mavjud envdan oladi. Konteynerlar orasidagi host, port va lokal TLS sozlamalari `compose.yaml`da berilgan. Hostdagi PostgreSQL talab qilinmaydi; eski host bazasidagi ma'lumotlar avtomatik ko'chirilmaydi.

```sh
docker compose config --quiet
docker compose up -d
docker compose ps
# PostgreSQL, PgBouncer va Redis healthy bo‘lgach:
go run .
# Boshqa terminalda:
curl -f http://127.0.0.1:8082/api/languages-active
```

Go loyiha terminalda ishlaydi. HTTP porti `.env`dagi `APP_PORT` qiymati (hozir `8082`). PostgreSQL tayyor bo'lgach PgBouncer ishga tushadi. `docker compose ps`da uchala xizmat `healthy` bo'lgach `go run .`ni bajaring. `go run .` bazaga PgBouncer orqali ulanadi va mavjud migratsiyalarni bajaradi. Go versiyasi haqiqiy `go.mod`ga mos `1.26` yoki undan yangi bo'lishi kerak.

PostgreSQL ma'lumotlari `postgres_data` named volumeda saqlanadi. `docker compose down`dan keyin ham qoladi; `down -v` bu ma'lumotlarni o'chiradi. Baza paroli va foydalanuvchi dastlab bo'sh volume yaratilganda o'rnatiladi; keyingi env o'zgarishi mavjud bazaning parolini avtomatik almashtirmaydi. [PostgreSQL image hujjati](https://hub.docker.com/_/postgres).

PostgreSQL hostga port ochmaydi. PgBouncer `127.0.0.1:${PGBOUNCER_PORT:-6432}`, Redis `127.0.0.1:${REDIS_PORT:-6380}` orqali ham mavjud. `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_SSLMODE` ushbu Compose ichidagi PostgreSQL uchun ishlatilmaydi. Mavjud `.env`dagi `DB_HOST=127.0.0.1`, `DB_PORT=6432`, `DB_SSLMODE=disable`, `REDIS_HOST=127.0.0.1`, `REDIS_PORT=6380` terminaldagi Go uchun mos. `DB_PORT` va `PGBOUNCER_PORT` bir xil bo‘lishi kerak.

PgBouncer `transaction` rejimida ishlaydi. `max_prepared_statements=200` pgx protocol prepared statementlarini qo'llaydi; pool `20`, zaxira `5`, bir baza chegarasi `25`, mijoz chegarasi `500`. Go `pgxpool` sozlamalari o'zgarmagan. [Image entrypoint sozlamalari](https://github.com/edoburu/docker-pgbouncer/blob/v1.24.1-p1/entrypoint.sh).

```sh
docker compose logs --tail 50 postgres pgbouncer redis
docker compose exec pgbouncer sh -c 'PGPASSWORD="$DB_PASSWORD" psql -h 127.0.0.1 -p 6432 -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1"'
docker compose stop
docker compose start --wait
```

Go kod o'zgargach terminaldagi serverni qayta ishga tushiring. `docker compose config`ni `--quiet`siz chiqarish parollarni ko'rsatishi mumkin.

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

## Avvalgi konfiguratsiyadagi tekshiruvlar

Quyidagi natijalar host PostgreSQL bilan avval bajarilgan; yangi Compose tekshiruvi sifatida qaralmaydi.

2026-11-12 kuni ikkala konteyner `healthy` bo'ldi. Haqiqiy Redis bilan vaqtinchalik tekshiruvlarda keshning bo'sh natijasi, keshdan o'qish, create/update/delete orqali yangilash, nom/faollik o'zgarishi, muvaffaqiyatsiz yozuvda keshni saqlash, eskirgan parallel yozuvni rad etish va Redis mavjud bo'lmaganda bazaga qaytish o'tdi. CRUD tekshiruvlari xotiradagi izolyatsiyalangan repository bilan bajarildi; mavjud PostgreSQL yozuvlari o'zgartirilmadi.

Haqiqiy PostgreSQLdan PgBouncer orqali faol tillar o'qildi; 8 parallel mijozda jami 240 parametrli pgx so'rovi muvaffaqiyatli bajarildi. `go build ./...` va `go vet ./...` o'tdi. Vaqtinchalik test fayli tekshiruvdan keyin olib tashlandi.

Yangilangan HTTP serverda `/api/languages-active` ikki marta `200` qaytardi; 5 ta faol til Redisdagi `test:golang:{languages}:active` kalitiga yozildi va 60 soniyalik TTL tasdiqlandi. Ishga tushishda yangi migratsiya qo'llanilmadi (`Migrations: nothing new`).

Redis mantiqi mavjud `lang_repo.go`ga birlashtirilgach, haqiqiy repository vaqtinchalik izolyatsiyalangan PostgreSQL va haqiqiy Redis bilan qayta tekshirildi: bo'sh ro'yxat, DBsiz cache hit, create/update/delete yangilashi, faol bo'lmagan til, muvaffaqiyatsiz o'chirish, buzilgan kesh JSONini tiklash, TTL va Redis mavjud bo'lmaganda SQLga qaytish o'tdi. Cache miss SQL so'rovi vaqtincha jadval locki bilan to'xtatilib, kesh versiyasi yangilangach eski o'qish yangi keshni almashtirmasligi ham tasdiqlandi. Vaqtinchalik konteyner va tekshiruv dasturi olib tashlandi.
