Status: done
Tanggal: 19 Nov 2026
Pemilik: tim LibraSense
Terkait: 002-firmware-push-19-nov-2026.md, 004-gap-fix-19-nov-2026.md

# Rangkaian lengkap 3 ruang: pin, catu daya, dan aktuator

## Konteks

Kebutuhan akhir adalah 3 ruang yang dipantau: satu ruang baca dan dua ruang rak. Setiap ruang punya satu DHT11 dan satu KY-037. Firmware sebelumnya memakai `RACK_COUNT 2`, sehingga ruang ketiga hanya tercatat sebagai cadangan di 002. Dokumen ini menetapkan pin final untuk 3 ruang, termasuk output dan catu daya, lalu mencatat perubahan firmware.

Board yang dipakai: ESP32 Development Board dengan expansion board, tiga breadboard, dan catu daya terpisah untuk 5 V dan 12 V.

## Pemetaan ruang

| Indeks protokol | Ruang | Label di panel | Pin sensor | Pin buzzer |
|---|---|---|---|---|
| 1 | Ruang Baca | `Ruang Baca` | DHT 23, KY-037 34 | 26 |
| 2 | Ruang Rak 1 | `Ruang Rak 1` | DHT 19, KY-037 35 | 25 |
| 3 | Ruang Rak 2 | `Ruang Rak 2` | DHT 13, KY-037 32 | 33 |

Kunci JSON tetap `rack` dan `b1` sampai `b3` supaya kontrak server dan penyimpanan tidak berubah. Artinya "ruang", bukan rak fisik.

## Prinsip pemilihan pin

- WiFi menonaktifkan ADC2. Pin 0, 2, 4, 12, 13, 14, 15, 25, 26, 27 tidak boleh dipakai untuk `analogRead` saat WiFi aktif. Sensor analog wajib di pin ADC1: 32, 33, 34, 35, 36, 39.
- Pin 34 sampai 39 hanya input, tidak punya pull-up internal. Cocok untuk keluaran analog KY-037, tidak cocok untuk buzzer, relay, atau DHT.
- Pin strapping 0, 2, 5, 12, 15 dihindari untuk output. GPIO12 juga MTDI, bisa menghalangi boot.
- Pin 6 sampai 11 dipakai flash internal. Pin 1 dan 3 dipakai UART0.
- Semua pin sinyal di bawah ini diambil dari silkscreen expansion board, bukan nomor urut header.

## Tabel pin final

| Indeks | Komponen | Terminal | GPIO ESP32 | Arah | Catatan |
|---|---|---|---|---|---|
| 1 | DHT11 | DATA | 23 | input | pull-up 10k ke 3V3 bila modul tidak punya |
| 2 | DHT11 | DATA | 19 | input | sama |
| 3 | DHT11 | DATA | 13 | input | sama |
| 1 | KY-037 | AO | 34 | input only | ADC1, tanpa pull-up internal |
| 2 | KY-037 | AO | 35 | input only | ADC1 |
| 3 | KY-037 | AO | 32 | input | ADC1 |
| 1 | Buzzer | lewat NPN | 26 | output | basis transistor, bukan buzzer langsung |
| 2 | Buzzer | lewat NPN | 25 | output | sama |
| 3 | Buzzer | lewat NPN | 33 | output | sama |
| - | Relay 1 kanal | IN | 27 | output | modul aktif rendah, default mati saat boot |
| - | LCD 16x2 I2C | SDA | 21 | I2C | pull-up ada di backpack |
| - | LCD 16x2 I2C | SCL | 22 | I2C | sama |

Pin yang dipakai: 12 GPIO. Sisa pin aman: 4, 16, 17, 18, 36, 39. Pin 36 dan 39 hanya input.

## Catu daya

| Rel | Sumber | Beban | Catatan |
|---|---|---|---|
| 5 V | adaptor 5 V 2 A | ESP32 VIN, relay VCC, buzzer +, LCD backpack bila memilih 5 V | arus WiFi ESP32 bisa 240 mA sesaat |
| 3,3 V | regulator board ESP32 | DHT11 x3, KY-037 x3, LCD backpack | dipakai agar AO KY-037 tidak melebihi 3,6 V |
| 12 V | adaptor 12 V 2 A | kipas DC lewat kontak relay | arus kipas tidak lewat breadboard |
| GND | GND ESP32, modul relay, sensor, LCD | - | negatif adaptor 12 V terpisah karena kontak relay kering |

Beban total 5 V: ESP32 240 mA, relay 70 mA, buzzer 3 x 30 mA, LCD 20 mA. Adaptor 2 A cukup.

Jack DC 6,5 sampai 12 V pada expansion board adalah masukan, bukan keluaran. Tegangan 12 V hanya ada di sisi masukan, lalu diturunkan regulator menjadi 5 V dan 3,3 V untuk header. Tidak ada pin 12 V di header. Kipas 12 V tetap butuh adaptor 12 V sendiri, atau satu adaptor 12 V yang dicabang ke jack board dan ke relay. Jangan memberi kipas 12 V dari pin 5 V board.

Bila hanya memakai satu adaptor 12 V untuk board dan kipas, pakai colokan DC female ke terminal sekrup, lalu cabang: satu ke jack board, satu ke COM relay. Polaritas colokan umumnya center positive, cek dengan multimeter sebelum disambung.

## Sambungan per komponen

### DHT11 (3 buah)

| Terminal modul | Tujuan |
|---|---|
| VCC | 3V3 |
| DATA | GPIO 23, 19, 13 sesuai ruang |
| GND | GND |

Modul DHT11 umumnya sudah membawa pull-up 10k. Bila memakai sensor telanjang, tambahkan resistor 10k dari DATA ke VCC.

### KY-037 (3 buah)

| Terminal modul | Tujuan |
|---|---|
| VCC | 3V3 |
| GND | GND |
| AO | GPIO 34, 35, 32 sesuai ruang |
| DO | tidak disambung, atau ke pin input bebas |

AO adalah keluaran analog yang naik turun mengikuti suara. DO adalah keluaran komparator digital: berubah LOW atau HIGH saat suara melewati batas yang disetel potensiometer di modul. Firmware hanya membaca AO, jadi DO boleh dibiarkan kosong.

Bila DO ingin tetap disambung untuk uji manual, pakai pin input bebas: 36, 39, 4, 16, 17, atau 18. Jangan pakai 34, 35, 32 karena sudah dipakai AO ketiga ruang. Jangan pakai 0, 2, 5, 12, 15 karena strapping. DO push-pull dari LM393, tidak butuh pull-up.

Aturan tegangan: DO mengikuti VCC modul. Pada VCC 3,3 V, DO aman langsung ke GPIO. Pada VCC 5 V, DO mencapai 5 V sehingga perlu pembagi tegangan atau tidak disambung sama sekali.

Dua pilihan catu daya:

1. VCC 3,3 V langsung ke AO. Aman untuk ADC, sensitivitas lebih rendah.
2. VCC 5 V dengan pembagi tegangan pada AO: resistor 10k seri dan 20k ke GND. Keluaran menjadi 5 x 20/30 = 3,33 V. Pilih ini bila suara ruangan terlalu kecil.

Jangan hubungkan AO 5 V langsung ke ESP32. ADC menerima maksimum sekitar 3,3 V.

### Buzzer (3 buah)

Buzzer aktif 5 V menarik sekitar 25 sampai 30 mA, melebihi arus aman GPIO. Pakai transistor NPN.

| Titik | Sambungan |
|---|---|
| GPIO 26, 25, 33 | resistor 1k ke basis BC547 atau 2N2222 |
| Emitor transistor | GND |
| Kolektor transistor | terminal negatif buzzer |
| Terminal positif buzzer | 5 V |

Buzzer aktif punya polaritas, terminal panjang atau tanda plus ke 5 V. Bila memakai modul buzzer yang sudah punya driver, pin sinyal modul langsung ke GPIO dan VCC ke 5 V.

Tanpa transistor, buzzer hanya boleh digerakkan langsung dari GPIO bila arusnya di bawah 12 mA, misalnya modul buzzer 3 pin dengan driver sendiri atau piezo kecil. Tambahkan resistor 100 ohm seri untuk membatasi arus. Ini solusi sementara untuk prototipe, bukan untuk pemakaian lama, karena arus lebih dari batas dapat melemahkan pin atau merusak GPIO.

### Relay dan kipas 12 V

| Terminal | Sambungan |
|---|---|
| VCC modul | 5 V |
| GND modul | GND bersama |
| IN | GPIO 27 |
| COM | positif adaptor 12 V |
| NO | positif kipas |
| NC | tidak dipakai |
| Negatif kipas | negatif adaptor 12 V |

Modul relay yang dipakai firmware dianggap aktif rendah: IN LOW berarti kipas menyala. Saat boot firmware menulis HIGH, jadi kipas tetap mati sampai server memerintahkan sebaliknya. Bila modul ternyata aktif tinggi, ubah `RELAY_ACTIVE_LOW` menjadi `false`.

Tambahkan dioda 1N4007 paralel dengan kipas, katoda ke positif, bila modul relay belum punya dioda flyback. Arus 12 V kipas disambung langsung ke terminal relay, bukan lewat jalur breadboard.

Modul relay aktif rendah memakai pull-up internal pada pin IN, tetapi tidak semua modul begitu. Bila kipas sempat berkedip saat ESP32 baru diberi daya, tambahkan resistor 10k dari IN ke 5 V agar pin pasti HIGH, artinya mati.

Terminal sekrup adalah blok dengan sekrup kecil penjepit kabel telanjang, misalnya blok hijau 2 pin 5,08 mm. Terminal COM, NO, dan NC pada modul relay juga terminal sekrup, jadi kabel kipas cukup dikupas 5 sampai 8 mm lalu dijepit di sana.

Kipas DC dua kabel memakai standar warna yang sama: merah positif, hitam negatif. Kabel female tidak bisa ditancap ke breadboard, jadi pakai jumper male ke male, terminal sekrup dua pin, atau potong konektor dan kupas 5 mm untuk dimasukkan ke terminal relay. Bila kipas berasal dari PC dan punya kabel kuning atau biru, keduanya tidak dipakai; kuning tachometer, biru PWM. Tanpa sinyal PWM kipas berputar penuh.

Urutan pengujian yang aman, tanpa kipas dan tanpa 12 V dulu:

1. Sambung VCC 5 V, GND, IN ke GPIO 27. Beri daya ESP32.
2. Dengar klik saat boot. Klik berarti relay sempat aktif sesaat. Bila sering, tambahkan resistor 10k tadi.
3. Ukur kontinuitas COM ke NO dengan multimeter. Saat pin HIGH harus terputus, saat LOW harus tersambung.
4. Buka panel, pilih mode kipas "Selalu nyala". Relay harus aktif tanpa perlu memanaskan sensor.
5. Baru sambung adaptor 12 V dan kipas ke COM dan NO. Jangan memakai beban 220 V AC pada prototipe ini.

### LCD 16x2 I2C

| Terminal backpack | Sambungan |
|---|---|
| VCC | 3V3 |
| GND | GND |
| SDA | GPIO 21 |
| SCL | GPIO 22 |

Backpack PCF8574 menerima 2,5 sampai 6 V. Pada 3,3 V backlight sedikit lebih redup tetapi jalur I2C tetap 3,3 V, sehingga aman untuk ESP32. Bila tetap memakai 5 V, pull-up pada backpack menarik SDA dan SCL ke 5 V dan itu di luar batas ESP32. Bila backlight terlalu redup pada 3,3 V, pakai level shifter I2C dua kanal dan catu 5 V.

Alamat I2C yang dicari firmware: 0x27 lalu 0x3F.

## Diagram

```
adaptor 5V 2A ---+------------------ VIN  ESP32  27 ---- IN relay
                 |                                 26 ---- 1k --- NPN --- buzzer 1
                 |                                 25 ---- 1k --- NPN --- buzzer 2
                 |                                 33 ---- 1k --- NPN --- buzzer 3
                 |                                 23 ---- DATA DHT11 ruang baca
                 |                                 19 ---- DATA DHT11 ruang rak 1
                 |                                 13 ---- DATA DHT11 ruang rak 2
                 |                                 34 <--- AO KY-037 ruang baca
                 |                                 35 <--- AO KY-037 ruang rak 1
                 |                                 32 <--- AO KY-037 ruang rak 2
                 |                                 21 <--> SDA LCD
                 |                                 22 <--> SCL LCD
                 |                                3V3 ---- VCC DHT x3, VCC KY-037 x3, VCC LCD
                 +-------------------------------- GND ---- GND ESP32, modul relay, sensor, LCD

adaptor 12V +  --- COM relay
                  NO relay --- kipas +
adaptor 12V -  --------------- kipas -
```

## Kesalahan yang harus dihindari

1. AO KY-037 dari catu 5 V langsung ke GPIO. Tegangan bisa mendekati 5 V dan merusak ADC.
2. LCD backpack 5 V tanpa level shifter. SDA dan SCL ikut naik ke 5 V.
3. Buzzer langsung dari GPIO tanpa transistor. Arus lebih dari batas dan pin bisa rusak.
4. Memakai GPIO 25, 26, 27 untuk sensor analog. Pin itu ADC2 dan tidak bekerja saat WiFi aktif.
5. Memakai GPIO 34 sampai 39 untuk output. Pin itu tidak bisa mengeluarkan tegangan.
6. Memakai GPIO 12 untuk buzzer. Pin strapping MTDI, bisa menggagalkan boot.
7. GND modul relay tidak disatukan dengan GND ESP32. Sinyal IN tidak punya referensi.
8. Arus kipas 12 V dilewatkan breadboard. Gunakan terminal relay.

## Perubahan firmware dan panel

- `RACK_COUNT` menjadi 3.
- `DHT_PIN` menjadi 23, 19, 13.
- `SUARA_PIN` menjadi 34, 35, 32.
- `BUZZER_PIN` menjadi 26, 25, 33.
- Tabel `ROOM_LABEL` berisi `Ruang Baca`, `Ruang Rak 1`, `Ruang Rak 2`. Label ini dikirim di `meta` dan dipakai panel, jadi sensor `dht1` bukan berarti rak fisik.
- Inisialisasi `DHT` ditambah ruang ketiga.
- Layar LCD halaman 1 menampilkan D1, D2, D3 dan status kipas. Halaman 2 menampilkan N1, N2, N3 dan status jaringan.
- Log serial menampilkan tiga suhu dan tiga nilai suara.
- Panel: judul kartu dan field pengaturan memakai nama ruang, bukan nomor rak. Perintah uji buzzer bertambah menjadi `test1`, `test2`, `test3`.
- Kontrak server tidak berubah karena `state.buzzer` dan `status.hot` sudah memakai tiga slot.

## Verifikasi

```
arduino-cli compile --fqbn esp32:esp32:esp32 esp32/librasense_node
```

Hasil: berhasil, sketch 1095804 byte (83 persen), variabel global 50648 byte (15 persen).

Unggah ke board pada 19 Nov 2026 lewat COM6 berhasil, hash terverifikasi. Serial 115200 menampilkan `WiFi tersambung: <DEVICE_IP>` dan baris `Suhu 28.9 29.8 29.3 | Suara 33 46 30`. Tiga DHT11 dan tiga KY-037 terbaca. Relay, kipas, dan buzzer belum dipasang, jadi tidak ada beban yang diuji.

## Deploy panel

Panel dan server di redeploy 19 Nov 2026 11:23 UTC lewat `deploy/deploy.ps1 -Remote`. Health mengembalikan `{"ok":true}`, kontainer aktif, dan aset yang dilayani sudah memuat label ruang, `test3`, serta `bm` default 7. Image sebelumnya disimpan sebagai `localhost/librasense:prev` untuk rollback.

Panel dibuka di `https://librasense.<domain>` dan wajib login Cloudflare Access. Hostname `iot.<domain>` hanya untuk perangkat: `/api/*` di sana selalu 401 karena tidak ada Access application, jadi panel akan tampil kosong bila dibuka dari hostname itu.

## Terbuka

- Ukur polaritas modul relay sebelum kipas disambung ke 12 V.
- Kalibrasi `bs` per ruang setelah KY-037 terpasang, karena keluaran analog bukan dB(A).
- Uji apakah KY-037 pada 3,3 V cukup sensitif di ruangan sebenarnya. Bila tidak, pakai 5 V dengan pembagi tegangan.
- Cek arus total 5 V saat ketiga buzzer berbunyi bersamaan.
- Hanya ada satu relay dan satu kipas. Server menyalakan kipas bila salah satu ruang melewati ambang, jadi kipas mendinginkan satu titik saja. Bila tiap ruang perlu kipas sendiri, tambah dua relay dan dua pin kontrol, lalu ubah kontrak `state` dari `fan` tunggal menjadi array.

## Rujukan

- `esp32/librasense_node/librasense_node.ino`
- `docs/002-firmware-push-19-nov-2026.md`
- `docs/004-gap-fix-19-nov-2026.md`
