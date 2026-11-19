# Panduan Presentasi dan Skenario Tanya Jawab (Q&A) LibraSense

Dokumen ini disusun sebagai panduan lengkap untuk presentasi proyek LibraSense, mencakup alur presentasi sistematis, naskah demonstrasi langsung (live demo), serta skenario tanya jawab (Q&A) teknis beserta strategi jawabannya.

---

## 1. Alur dan Naskah Presentasi (Durasi Estimasi: 7–10 Menit)

### Bagian 1: Pembuka dan Latar Belakang (1 Menit)
- **Poin Utama**: Mengapa sistem ini dibuat?
- **Naskah**:
  > "Selamat pagi/siang Bapak/Ibu dosen penguji dan rekan-rekan sekalian. Pada kesempatan kali ini, kami mempresentasikan proyek IoT kami yang berjudul **LibraSense: Sistem Monitoring Lingkungan dan Otomasi Perpustakaan Berbasis IoT**.
  >
  > Latar belakang proyek ini berfokus pada dua isu utama perpustakaan:
  > 1. **Kenyamanan pengunjung**: Menjaga tingkat kebisingan di ruang baca agar tetap tenang dan kondusif.
  > 2. **Preservasi koleksi buku**: Mengontrol suhu dan kelembapan di ruang rak penyimpanan untuk mencegah pertumbuhan jamur dan kerapuhan serat kertas akibat udara lembap atau panas ekstrem.
  >
  > Untuk menyelesaikan masalah tersebut, LibraSense mengintegrasikan node sensor ESP32, server Go mandiri dengan embedded database SQLite, serta panel dasbor interaktif dengan visualisasi denah maket perpustakaan."

---

### Bagian 2: Arsitektur Sistem dan Rekayasa Utama (2.5 Menit)
- **Poin Utama**: Apa keunikan dan keunggulan teknis sistem ini?
- **Naskah**:
  > "Arsitektur LibraSense dibangun dengan prinsip **Decoupled IoT Architecture**:
  > - **ESP32 berfungsi sebagai 'Dumb Node'**: Mikrokontroler hanya bertugas mengumpulkan data fisik sensor, mengemas payload JSON, mengirim via HTTP POST, dan menerapkan instruksi keluaran pin. Di dalam firmware ESP32, sama sekali tidak ada logika ambang batas atau histeresis.
  > - **Server Go sebagai 'Smart Decision Engine'**: Seluruh perhitungan, evaluasi ambang batas suhu per ruang, histeresis kipas, dan status alarm dihitung terpusat di server pada modul `internal/decision`. Ini memastikan bila ada perubahan regulasi batas suhu, pengelola cukup mengubah pengaturan di web tanpa perlu membongkar atau mem-flash ulang mikrokontroler.
  > - **Single Static Binary**: Backend ditulis dengan Go murni menggunakan SQLite murni (`modernc.org/sqlite`, `CGO_ENABLED=0`). Seluruh aset frontend (HTML, CSS, Alpine.js) di-embed langsung ke dalam satu file binary executable. Server ini berjalan sangat ringan di dalam kontainer Podman pada VPS bersama.
  > - **Keamanan Berlapis**: Komunikasi perangkat diproteksi Bearer Token dengan hashing konstan, sedangkan akses dasbor diproteksi Cloudflare Zero Trust Access."

---

### Bagian 3: Implementasi Perangkat Keras dan Maket (1.5 Menit)
- **Poin Utama**: Komponen fisik apa saja yang digunakan?
- **Naskah**:
  > "Prototipe fisik dibuat dalam bentuk maket skala 3 ruangan:
  > 1. **Ruang Baca (Ruang Utama)**: Dipantau oleh sensor DHT11 dan sensor suara KY-037.
  > 2. **Ruang Rak 1**: Dipantau oleh DHT11 dan KY-037 kedua.
  > 3. **Ruang Rak 2**: Dipantau oleh DHT11 dan KY-037 ketiga.
  >
  > Seluruh sensor suara analog dialokasikan pada blok pin ADC1 ESP32 (GPIO 34, 35, 32) untuk menghindari konflik modul WiFi yang secara otomatis mematikan blok ADC2.
  >
  > Untuk aktuasi pendingin, sistem menggunakan modul relay 1-kanal (GPIO 27) pengendali kipas 12V. Status luring di dekat maket juga dapat dipantau langsung melalui layar LCD 16x2 I2C."

---

### Bagian 4: Demonstrasi Langsung / Live Demo (2.5 Menit)
- **Langkah 1 (Kondisi Normal)**:
  > "Bisa kita lihat pada dasbor, saat ini ketiga ruangan dalam status Normal (hijau). Angka suhu berada di kisaran 29 °C dan tingkat kebisingan berada di ambang tenang (skala 20–30 ADC). Denah maket SVG mereplikasi tata letak kardus fisik lengkap dengan pintu masuk dan meja kursi."
- **Langkah 2 (Pengujian Sensor Kebisingan)**:
  > *(Tepuk tangan atau buat suara keras di dekat mikrofon KY-037)*
  > "Saat kita membuat suara keras, dalam interval siklus 5 detik data dikirim ke server. Nilai kebisingan melonjak melampaui ambang batas, dan pada dasbor status ruang langsung berubah menampilkan tanda peringatan berisik."
- **Langkah 3 (Pengujian Suhu dan Kipas)**:
  > *(Ubah nilai ambang batas b1/b2 di panel kontrol pengaturan menjadi lebih rendah dari suhu saat ini, misal 28 °C, lalu klik Simpan)*
  > "Ketika ambang batas suhu kita turunkan di bawah suhu aktual, server mengevaluasi kondisi panas (`hot=true`). Server memerintahkan kipas aktif, dan relay kipas langsung menyala, dibuktikan dengan indikator LED merah pada modul relay dan putaran kipas."
- **Langkah 4 (Layar LCD)**:
  > "Di saat bersamaan, layar LCD di samping maket bergantian menampilkan data telemetri ketiga ruang dan status kipas tanpa bergantung pada koneksi dasbor komputer."

---

### Bagian 5: Kesimpulan dan Penutup (0.5 Menit)
- **Naskah**:
  > "Kesimpulannya, sistem LibraSense berhasil membuktikan bahwa pemantauan lingkungan perpustakaan multi-ruangan dapat diimplementasikan secara efisien, stabil, aman, dan mudah dikelola melalui arsitektur terpusat. Sekian presentasi dari kami, kami persilakan jika ada pertanyaan dari Bapak/Ibu dosen penguji. Terima kasih."

---

## 2. Skenario Tanya Jawab Teknis (Q&A)

### Kategori A: Arsitektur & Perangkat Lunak (Software & Architecture)

**Q1: Mengapa mikrokontroler tidak diberi wewenang mengambil keputusan sendiri (misalnya langsung menyalakan kipas saat suhu naik)? Mengapa harus lewat server?**
- **Jawaban**:
  > "Ada dua pertimbangan utama:
  > 1. **Kemudahan Pemeliharaan (Maintainability)**: Jika logika ada di mikrokontroler, setiap perubahan batas regulasi suhu atau pergantian algoritma histeresis mengharuskan teknisi menyambungkan kabel dan mem-flash ulang firmware di lapangan. Dengan arsitektur terpusat di server Go, pustakawan cukup mengubah parameter di antarmuka web, dan seluruh node otomatis mengikuti tanpa downtime.
  > 2. **Pencegahan Flapping**: Server memiliki data historis dan logika histeresis terintegrasi multi-ruang untuk memastikan kipas tidak mati-hidup secara kasar saat suhu berfluktuasi tipis di sekitar garis batas."

**Q2: Mengapa memilih bahasa Go dan SQLite dibandingkan stack populer seperti Node.js + Express + MongoDB atau Python?**
- **Jawaban**:
  > "Proyek ini dirancang untuk berjalan di lingkungan VPS bersama (shared VPS) dengan kapasitas RAM dan CPU terbatas.
  > - Go menghasilkan satu file binary mandiri dengan jejak memori (RAM) sangat kecil (kurang dari 30 MB) serta waktu startup instan.
  > - SQLite murni Go (`modernc.org/sqlite`) tidak membutuhkan daemon database eksternal yang boros memori seperti MySQL atau Postgres. Konfigurasi mode WAL (Write-Ahead Logging) memberikan performa konkurensi baca-tulis yang sangat cepat untuk kebutuhan telemetri sensor.
  > - Panel frontend di-embed langsung (`//go:embed`), sehingga tidak ada ketergantungan runtime Node.js atau ribuan berkas `node_modules` di server."

**Q3: Bagaimana aspek keamanan sistem ini dilindungi dari pihak luar?**
- **Jawaban**:
  > "Kami menerapkan autentikasi dua jalur:
  > 1. Jalur transmisi perangkat ESP32 (`/device/*`) diamankan menggunakan HTTP Bearer Token 32-byte acak. Server memvalidasi token menggunakan pencocokan waktu-konstan (`constant-time compare`) untuk mencegah serangan *timing attack*.
  > 2. Jalur antarmuka pengguna (`/api/*` dan web) diproteksi menggunakan **Cloudflare Zero Trust Access**. Pengguna wajib melewati autentikasi OTP email organisasi, dan server Go memverifikasi kriptografi JSON Web Token (JWT) dari Cloudflare."

---

### Kategori B: Perangkat Keras & Sensor (Hardware & Electronics)

**Q4: Mengapa semua sensor suara KY-037 dipasang di pin GPIO 34, 35, dan 32? Kenapa tidak di pin analog sembarang?**
- **Jawaban**:
  > "ESP32 memiliki dua blok konverter analog-ke-digital: ADC1 dan ADC2. Ketika modul radio WiFi ESP32 aktif mentransmisikan data, chip ESP32 secara internal mematikan modul ADC2. Oleh karena itu, pin-pin yang terhubung ke ADC2 (seperti GPIO 2, 4, 12, 14, 15, 25, 26, 27) tidak dapat digunakan untuk pembacaan `analogRead()`. Kami wajib mengalokasikan sensor suara analog ke pin blok ADC1 (GPIO 32, 34, 35) agar pembacaan sensor tidak korup saat terkoneksi ke WiFi."

**Q5: Mengapa modul relay menggunakan relay Active LOW? Apa pengaruhnya saat ESP32 baru dinyalakan (booting)?**
- **Jawaban**:
  > "Modul relay yang beredar di pasaran umumnya aktif ketika sinyal masukan bernilai logika LOW (0V). Pada saat awal booting, pin GPIO ESP32 berada pada status floating/input. Untuk mencegah relay dan kipas menyala secara liar (glitch) saat perangkat menyala ulang, pada blok `setup()` firmware kami langsung mengatur pin GPIO 27 ke status `HIGH` sebelum mengaktifkan mode `OUTPUT`."

**Q6: Bagaimana cara kerja sensor kebisingan KY-037 membaca suara? Apakah nilainya setara dengan desibel (dB)?**
- **Jawaban**:
  > "Nilai yang dibaca modul KY-037 adalah nilai tegangan analog mentah (skala ADC 12-bit, rentang 0–4095). Modul ini menggunakan mikrofon kondensor dan penguat op-amp LM393.
  > Firmware kami membaca jendela cuplik selama 50 milidetik, lalu menghitung selisih nilai puncak ke lembah (*peak-to-peak = high - low*). Nilai ini bukan desibel (dB) terkalibrasi laboratorium, melainkan representasi amplitudo lonjakan suara relatif yang efektif membedakan suasana tenang perpustakaan dengan suara berisik atau benturan."

**Q7: Mengapa prototipe maket ini tidak memasang modul buzzer fisik?**
- **Jawaban**:
  > "Pada arsitektur sistem, jalur logika alarm kebisingan dan suhu panas per ruang telah diimplementasikan penuh di modul `internal/decision` dan firmware ESP32 (mendukung bitmask pengujian `test1`, `test2`, `test3`). Namun pada maket fisik, komponen buzzer sengaja ditiadakan untuk menjaga ketenangan lingkungan simulasi maket agar tidak menghasilkan polusi suara melengking selama pengujian berulang dan presentasi. Sinyal aktuasi alarm tetap dapat diverifikasi melalui status respons telemetri dasbor."

---

### Kategori C: Pengujian & Kehandalan (Reliability & Troubleshooting)

**Q8: Apa yang terjadi jika koneksi WiFi terputus atau server mati?**
- **Jawaban**:
  > "Sistem memiliki mekanisme fail-safe:
  > 1. Firmware ESP32 memiliki algoritma *exponential backoff*: jika permintaan HTTP gagal, ESP32 tidak membanjiri jaringan dengan request berulang, melainkan menaikkan interval tunggu secara bertahap hingga koneksi pulih.
  > 2. Jika koneksi jaringan mati, sistem lokal tetap bekerja: layar LCD 16x2 tetap menampilkan suhu dan status luring secara mandiri.
  > 3. Dasbor web akan menandai perangkat sebagai 'Luring' (*offline*) jika pembacaan terakhir melewati batas toleransi waktu tanpa data baru."

**Q9: Saat mengamati grafik riwayat suhu, mengapa garisnya tampak mendatar selama beberapa menit? Apakah datanya macet?**
- **Jawaban**:
  > "Bukan macet. Berdasarkan analisis database langsung, sensor DHT11 memiliki resolusi pembacaan bertingkat 1.0 °C. Pada kondisi ruangan tertutup yang suhunya stabil (misalnya 28.9 °C), nilai sensor tidak akan melompat setiap detik. Sementara itu, sensor kebisingan KY-037 yang memiliki resolusi halus membuktikan bahwa data terus mengalir dinamis setiap 5 detik dengan nilai yang berganti secara kontinu."
