# Alohida news stress testi

API serverni o'zingiz ishga tushiring. Boshqa terminalda loyiha papkasidan:

```powershell
go run ./stress-test/main.go
```

Terminal quyidagilarni so'raydi:

```text
Parallel userlar soni [10000]: 20000
Qancha vaqt ishlasin? (1h, 20m, 30s yoki daqiqa) [1h]: 1h
```

Userlar soniga `10000`, `20000`, `100000` yoki boshqa musbat son kiriting. Davomiylikka `1h`, `20m`, `30s` yozish mumkin; faqat `60` kiritsangiz 60 daqiqa deb olinadi. Enter bosilsa standart qiymatlar: **10 000 user, 1 soat**. Noto'g'ri qiymat qayta so'raladi. Ikkala javob kiritilgandan keyin tayyorlash va yuklama boshlanadi.

Hozirgi `.env`ga mos standart server manzili `http://localhost:8082`. Boshqa port uchun:

```powershell
go run ./stress-test/main.go -url http://localhost:8080
```

Papka ichidan ham ishga tushirish mumkin:

```powershell
cd stress-test
go run main.go
```

Test server bilan faqat HTTP orqali ishlaydi. Server kodi, ulanish sozlamalari va migratsiyalarni o'zgartirmaydi. Mavjud request DTOlarini ishlatadi; yangi kutubxona o'rnatish kerak emas.

## Ishlash tartibi

1. `load-uz`, `load-ru`, `load-en`, `load-de`, `load-fr` nomli 5 ta aktiv til yaratadi. Shu nomdagi aktiv tillar mavjud bo'lsa qayta ishlatadi.
2. Testga xos nomlar bilan 10 ta aktiv kategoriya yaratadi.
3. Show uchun mavjud ID bo'lishi maqsadida bitta boshlang'ich news yaratadi.
4. Kiritilgan userlar soni bilan kiritilgan vaqt davomida tanaffussiz ishlaydi.

| Amal | Ulush | HTTP |
| --- | --- | --- |
| Create | 50% | `POST /api/news` |
| List | 25% | `GET /api/news` |
| Show | 25% | `GET /api/news/:id` |

Umumiy navbat har to'rt so'rov uchun `create, list, create, show` ajratadi. Shu sabab endpointlar tezligi farq qilsa ham yuborilgan urinishlarda nisbat saqlanadi. Oxirgi tugallanmagan to'rtlikda farq ko'pi bilan bitta so'rov. Xatolar bo'lsa muvaffaqiyatli so'rovlar nisbati farq qilishi mumkin. Tayyorlash yuklama vaqtiga va statistikasiga kirmaydi.

Har virtual user bir vaqtda bitta so'rov yuboradi, javobdan so'ng keyingisini boshlaydi. `10000` user 10000 RPS degani emas: tezlik server javobiga ham bog'liq. Global nisbat saqlanadi; har bir userning alohida amallar nisbati shart emas. Loyihada account/auth moduli yo'q: account yaratilmaydi, virtual userning `1..users` IDsi newsdagi `user_id`ga yoziladi.

Har yangilik bitta tayyorlangan kategoriyaga ulanadi va barcha aktiv tillarda tarjimasi bo'ladi. Oldindan aktiv tillar mavjud bo'lsa ular ham tarjimalarga qo'shiladi; mavjud tillar o'zgartirilmaydi. Shu sabab umumiy aktiv tillar soni 5 tadan ko'p bo'lishi mumkin. Har tarjimada 256 bayt description yuboriladi. Sinov davomida til faolligini o'zgartirish create xatolarini keltirishi mumkin.

List til/kategoriya bo'yicha `limit=10` bilan ishlaydi, `next_cursor` orqali keyingi sahifaga o'tadi va oxirida boshiga qaytadi. Show haqiqiy create/list javobidagi IDni yoki boshlang'ich news IDsi bilan ishlaydi.

## Natijalar va to'xtatish

Har 10 sekundda tugagan, xato, bekor qilingan va jarayondagi so'rovlar hamda oxirgi interval RPSi terminalga chiqadi. Oxirida create/list/show uchun ulush, xatolar, o'rtacha/p95/maksimal javob vaqtlari, HTTP statuslar va ko'pi bilan 5 tadan xato namunasi ko'rsatiladi. Status `0` HTTP javob olinmaganini bildiradi. Buzilgan JSON va noto'g'ri news IDsi ham xato hisoblanadi.

Vaqt tugagach yangi ishlar olinmaydi; boshlangan so'rovlar standart 30 sekundlik timeoutgacha yakunlanadi. Yakuniy vaqt va o'rtacha RPS shu kutishni ham qamrab oladi. `Ctrl+C` so'rovlarni bekor qiladi va hisobot chiqaradi. Muvaffaqiyatli test exit code `0`, xato yoki to'xtatish `1` qaytaradi.

Latency payload tayyorlash, ulanish kutish, HTTP javobini to'liq o'qish va JSONni tekshirishni qamrab oladi. p95 doimiy hajmdagi logarifmik histogramdan taxminan 10% aniqlikdagi yuqori chegara sifatida olinadi. Barcha so'rovlar yoki barcha news IDlari xotirada yig'ilmaydi.

Ixtiyoriy JSON hisobot va timeout:

```powershell
# Loyiha papkasidan; userlar va vaqt baribir terminalda so'raladi
go run ./stress-test/main.go -timeout 60s -out ./stress-test/results.json
```

Bir xil hisobot nomi qayta ishlatilsa avvalgi fayl almashtiriladi.

Test haqiqiy bazaga yozadi; yaratilgan ma'lumotlarni avtomatik o'chirmaydi. 20 ming/100 ming userda yuklama yuboruvchi kompyuterning CPU/RAM, TCP portlari va tarmoq cheklovlari ham natijaga ta'sir qiladi. `100000` kiritish shu miqdorda muvaffaqiyatli TCP ulanish kafolati emas. Katta yuklama uchun generatorni alohida kompyuterda ishga tushirish mumkin. Bir bazaga bir nechta generatorni bir paytda boshlash til tayyorlashida parallel o'zgarishlarga olib kelishi mumkin.
