# News View stress testi

Bu mustaqil Go dastur. `show-redis` papkasini loyihadan tashqariga ko'chirib ham ishlatish mumkin. Faqat Go 1.24.4 yoki yangiroq versiya kerak; tashqi kutubxona, loyiha kodi, Redis/DB ulanishi yoki `.env` kerak emas. API server alohida ishlab turadi.

Terminalda shu papkaga kiring:

```powershell
cd stress-test/show-redis
go run .
```

Standart rejim faqat `GET /api/news/:id/view` so'rovlarini yuboradi. Terminalda to'rtta qiymat so'raladi:

```text
Parallel userlar soni [100]: 1000
Qancha vaqt ishlasin? (10m, 30s yoki daqiqa) [10m]: 10
Boshlanish news ID [1]: 1
Oxirgi news ID [10000]: 10000
```

Bu 1000 ta parallel virtual user bilan 10 daqiqa davomida `1..10000` oralig'idagi yangiliklarni ochadi. Ikkala chegara ham oraliqqa kiradi. Enter bosilsa 100 user, 10 daqiqa va `1..10000` olinadi. Faqat raqam bilan yozilgan interaktiv vaqt daqiqa hisoblanadi; `30s`, `10m`, `1h` ham mumkin.

Qiymatlarni flaglarda bersangiz, terminalda qayta so'ralmaydi:

```powershell
# 1..10000 ID, 1000 user, 10 daqiqa
go run . -users 1000 -duration 10m -min-id 1 -max-id 10000

# 563796 gacha ID
go run . -users 1000 -duration 10m -min-id 1 -max-id 563796

# 1 million gacha ID
go run . -users 1000 -duration 10m -min-id 1 -max-id 1000000

# 2 million gacha ID, userlarni dastlabki 10 sekundda bosqichma-bosqich boshlash
go run . -users 1000 -duration 10m -min-id 1 -max-id 2000000 -ramp 10s

# Ketma-ket IDlar va JSON hisobot
go run . -users 1000 -duration 10m -min-id 1 -max-id 2000000 -id-mode sequential -out ./view-results.json
```

| Parametr | Ma'nosi |
| --- | --- |
| `-users` | Bir vaqtda ishlaydigan virtual userlar soni |
| `-duration` | Davomiylik: `10m`, `30s`, `1h`; berilmasa so'raladi |
| `-min-id` / `-max-id` | IDning boshlanish/oxirgi chegarasi; berilmaganlari so'raladi |
| `-id-mode random` | Standart: har so'rovga tasodifiy ID tanlanadi, takrorlanishi mumkin |
| `-id-mode sequential` | Barcha userlar umumiy ketma-ketlikdan ID oladi, oxiriga yetgach boshidan boshlaydi |
| `-ramp 10s` | Userlar 10 sekund davomida ishga qo'shiladi; bu vaqt test davomiyligiga kiradi |
| `-url` | Standart `http://localhost:8082`; `/api` qo'shmang |
| `-timeout 30s` | Har bir HTTP so'rovning eng ko'p kutish vaqti |
| `-out` | Ixtiyoriy JSON hisobot; mavjud fayl bo'lsa almashtiriladi |

**Yangiliklar soni va eng katta ID bir xil bo'lishi shart emas.** Masalan, bazada 563796 ta yozuv bo'lsa ham oxirgi ID kattaroq bo'lishi mumkin. O'chirilgan IDlar oralig'ida bo'shliqlar bo'lsa, `404` xato hisoblanadi. Test IDlarni oldindan bazadan yuklamaydi va yetishmagan yangiliklarni yaratmaydi.

View rejimida til, kategoriya yoki news yaratish bosqichi yo'q. Biroq View endpointining o'zi hisoblagichni oshiradi: test so'rovlari Redisdagi ko'rishlar soniga qo'shiladi. Javob `200`, so'ralgan ID va musbat `view_count` bilan kelishi tekshiriladi. Bu test hisoblarning yakuniy DB/Redis bilan tengligini tekshirmaydi.

`1000 user` — bir vaqtda ko'pi bilan 1000 ta so'rov; 1000 RPS yoki 1000 ta account degani emas. Har user javobni kutib, keyingi so'rovni yuboradi. 1–2 million ID uchun alohida user yoki ID massivi yaratilmaydi: xotira sarfi asosan userlar, ochiq ulanishlar va javob hajmiga bog'liq. Tasodifiy rejim hamma IDga yetib borishni kafolatlamaydi. Ketma-ket rejimda ham hammasini bir marta ko'rish uchun kamida ID oralig'i hajmicha so'rov yuborilishi kerak.

p95/p99 doimiy hajmdagi histogramdan taxminan 10% aniqlikdagi yuqori chegara sifatida hisoblanadi. Xato bo'lsa exit code `1`, muvaffaqiyatli yakunda `0` qaytadi.

Har 10 sekundda RPS, tugagan va xato so'rovlar chiqariladi. Yakunda jami/muvaffaqiyatli RPS, xato foizi, o'rtacha/p95/p99/maksimal vaqtlar, HTTP statuslar va xato namunalari ko'rsatiladi. Vaqt tugaganda yangi so'rov yuborilmaydi, boshlanganlari timeoutgacha tugatiladi. `Ctrl+C` to'xtatadi. Generatorning CPU/RAM va TCP port cheklovlari ham natijaga ta'sir qiladi; shu sabab millionlab yangilik borligi million parallel user talab qilishini bildirmaydi.


Go o'rnatilmagan kompyuterda ishlatish uchun executable yig'ish mumkin:

```powershell
go build -o show-redis.exe .
.\show-redis.exe -url http://127.0.0.1:8082 -users 1000 -duration 10m -min-id 1 -max-id 2000000
```

Executable faylni shu operatsion tizim va arxitekturadagi boshqa kompyuterga ko'chirib ishlatsa bo'ladi.
