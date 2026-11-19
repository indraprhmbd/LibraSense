# Konteks Teknis dan Ringkasan Rekayasa Proyek LibraSense

Dokumen ini disusun sebagai ringkasan teknis komprehensif tanpa hiasan untuk keperluan penyusunan laporan, analisis arsitektur, dan pembuatan dokumen teknis menggunakan model bahasa (LLM) atau penyusunan laporan akademik.

---

## 1. Identitas dan Tujuan Proyek

- **Nama Proyek**: LibraSense
- **Kategori**: Sistem Internet of Things (IoT), Pemantauan Lingkungan Waktu-Nyata, dan Otomasi Kontrol Gedung (Smart Library).
- **Tujuan Utama**:
  1. Memantau parameter kualitas lingkungan perpustakaan secara waktu-nyata pada 3 ruangan: satu Ruang Baca (Ruang Utama) dan dua Ruang Koleksi Rak Buku (Ruang Buku 1 dan 2).
  2. Mencegah degradasi fisik koleksi buku (jamur, pembusukan kertas, kerapuhan) akibat suhu dan kelembapan ekstrem.
  3. Menjaga kenyamanan dan ketenangan ruang baca dari polusi kebisingan suara pengunjung.
  4. Mendukung kontrol otomasi aktuator (kipas ventilasi 12V dan alarm buzzer aktif) secara terpusat berdasarkan logika terprogram, dengan arsitektur modular yang tetap berjalan penuh sebagai sistem pemantauan murni (*monitoring-only*) apabila modul aktuasi fisik ditiadakan.
  5. Menyediakan antarmuka visual berupa dasbor web responsif dengan denah maket interaktif yang merefleksikan tata letak fisik secara akurat.

---

## 2. Prinsip Rekayasa dan Aturan Inti Arsitektur

Arsitektur LibraSense dibangun dengan prinsip pemisahan tanggung jawab secara tegas (*Strict Decoupled IoT Architecture*):

1. **Mikrokontroler Dumb, Server Pintar**:
   - Mikrokontroler ESP32 hanya bertugas mengumpulkan data mentah sensor, mengemas JSON, mengirim via HTTP POST, serta menerapkan status keluaran aktuator yang diinstruksikan oleh server.
   - Tidak ada penetapan ambang batas (*threshold*), histeresis, atau logika pembuatan keputusan di dalam firmware ESP32.
   - Seluruh logika evaluasi dan penentuan aktuasi berpusat di server pada modul `internal/decision`.
2. **Single Static Binary (Go + SQLite)**:
   - Backend dibangun dengan bahasa Go murni menggunakan driver SQLite murni Go (`modernc.org/sqlite`).
   - Kompilasi `CGO_ENABLED=0` menghasilkan satu binary statis mandiri tanpa ketergantungan runtime libc eksternal, dapat berjalan di atas image kontainer *distroless* atau *scratch*.
   - Database SQLite dikonfigurasi dengan mode WAL (*Write-Ahead Logging*) dan satu koneksi tulis (`SetMaxOpenConns(1)`) untuk mencegah persaingan kunci (*table lock contention*).
3. **Zero Frontend Build Step**:
   - Panel dasbor web tidak menggunakan bundler eksternal (tanpa npm, Vite, Webpack, Node.js, atau React).
   - Antarmuka hanya menggunakan HTML5 semantik, CSS murni dengan variabel desain (*design tokens*), pustaka Alpine.js 3.14.8 yang disimpan lokal (*vendored*), serta grafik vektor SVG.
   - Seluruh aset web ditanam langsung ke dalam executable server melalui direktori `web/` menggunakan fitur `//go:embed`.
4. **Keamanan Berlapis (Dual-Path Ingress Authentication)**:
   - **Jalur Perangkat (`/device/*`)**: Menggunakan autentikasi Bearer Token 32-byte acak. Server memvalidasi token melalui perbandingan hash SHA-256 menggunakan algoritma *constant-time compare* (`subtle.ConstantTimeCompare`) untuk mencegah serangan *timing attack*.
   - **Jalur Pengguna/Panel (`/api/*`)**: Diamankan menggunakan Cloudflare Zero Trust Access. Setiap permintaan API wajib membawa token JWT Cloudflare Access valid yang diverifikasi terhadap kunci publik JWKS organisasi (`golang-jwt/jwt/v5`).
   - Server menganut prinsip *fail-closed*: server menolak menyala di lingkungan produksi jika kunci konfigurasi keamanan tidak lengkap.

---

## 3. Topologi Perangkat Keras dan Elektronika

Sistem fisik memantau 3 ruangan yang direpresentasikan dalam maket kardus 3D proporsional:
- **Ruang 1**: Ruang Baca (Ruang Utama)
- **Ruang 2**: Ruang Rak 1 (Ruang Buku 1)
- **Ruang 3**: Ruang Rak 2 (Ruang Buku 2)

### 3.1 Daftar Komponen

Sistem dirancang modular: komponen sensor dan mikrokontroler merupakan subsistem inti pemantauan (*core sensing*), sedangkan modul aktuator (relay, kipas, buzzer) bersifat periferal opsional yang dapat dipasang atau dilepas tanpa memengaruhi kestabilan pemantauan data.

| Komponen | Spesifikasi / Tipe | Jumlah | Status Implementasi | Peran Teknis |
|---|---|:---:|:---:|---|
| Mikrokontroler | ESP32 DOIT DevKit V1 (30-pin) | 1 unit | Inti | Pemroses utama, pembaca sinyal, transmisi data |
| Board Ekspansi | ESP32 Expansion Shield | 1 unit | Inti | Distribusi pin header sinyal, VCC (5V/3.3V), dan GND |
| Sensor Suhu & Kelembapan | DHT11 Digital Module | 3 unit | Inti | Mengukur suhu (0–50 °C) dan kelembapan (20–90 % RH) di 3 ruang |
| Sensor Kebisingan | KY-037 Sound Microphone Module | 3 unit | Inti | Mengukur fluktuasi amplitudo gelombang suara analog di 3 ruang |
| Display Lokal | LCD Karakter 16x2 + I2C Backpack | 1 unit | Inti | Monitor teks status luring di samping maket fisik |
| Catu Daya Sistem | Dual Power Supply (5V & 12V) | 1 paket | Inti | Adaptor 5V/2A untuk logika dan adaptor 12V/1A untuk beban kipas |
| Modul Relay | Relay 5V 1-Kanal (Active LOW) | 1 unit | Opsional / Periferal | Sakelar switching daya DC 12V untuk kipas pendingin |
| Kipas Pendingin | DC Brushless Fan 12V | 1 unit | Opsional / Periferal | Sirkulasi udara pembuang panas rak koleksi |
| Indikator Suara | Active Buzzer 5V | 3 unit | Opsional / Periferal | Alarm peringatan suara per ruang via driver transistor NPN |
| Transistor Driver | NPN (2N2222 / SS8050) | 3 unit | Opsional / Periferal | Penguat arus penggerak buzzer dari pin GPIO 3.3V |

### 3.2 Alokasi Pin GPIO ESP32

Pemilihan GPIO mempertimbangkan batasan silikon ESP32:
- Pin analog ADC1 wajib digunakan untuk sensor analog karena modul radio WiFi menonaktifkan seluruh pin ADC2 ketika aktif.
- Pin strapping (GPIO 0, 2, 5, 12, 15) dan pin SPI Flash (GPIO 6–11) dihindari.
- GPIO 34–39 bersifat *input-only* tanpa resistor pull-up internal.

| Ruang / Indeks | Perangkat | Jalur Pin | GPIO ESP32 | Arah | Keterangan Sirkuit |
|---|---|---|:---:|:---:|---|
| Ruang 1 (Baca) | DHT11 (1) | Data | GPIO 23 | Input | Membaca sinyal digital 1-wire |
| Ruang 1 (Baca) | KY-037 (1) | Analog (AO) | GPIO 34 | Input Only | ADC1 kanal 6 (bebas konflik WiFi) |
| Ruang 1 (Baca) | Buzzer (1) | Basis Driver | GPIO 26 | Output | Memicu transistor NPN buzzer 1 |
| Ruang 2 (Rak 1) | DHT11 (2) | Data | GPIO 19 | Input | Membaca sinyal digital 1-wire |
| Ruang 2 (Rak 1) | KY-037 (2) | Analog (AO) | GPIO 35 | Input Only | ADC1 kanal 7 (bebas konflik WiFi) |
| Ruang 2 (Rak 1) | Buzzer (2) | Basis Driver | GPIO 25 | Output | Memicu transistor NPN buzzer 2 |
| Ruang 3 (Rak 2) | DHT11 (3) | Data | GPIO 13 | Input | Membaca sinyal digital 1-wire |
| Ruang 3 (Rak 2) | KY-037 (3) | Analog (AO) | GPIO 32 | Input | ADC1 kanal 4 (bebas konflik WiFi) |
| Ruang 3 (Rak 2) | Buzzer (3) | Basis Driver | GPIO 33 | Output | Memicu transistor NPN buzzer 3 |
| Global | Relay Kipas | Signal (IN) | GPIO 27 | Output | Mengendalikan relay kipas 12V (Active-LOW) |
| Global | LCD 16x2 | I2C SDA | GPIO 21 | I/O | Komunikasi I2C wire data |
| Global | LCD 16x2 | I2C SCL | GPIO 22 | Output | Komunikasi I2C clock (alamat `0x27`) |

### 3.3 Catatan Khusus Pengkondisian Sensor KY-037
- Modul KY-037 membaca sinyal audio melalui mic kondensor dan op-amp LM393. Potensiometer pada modul hanya mengatur komparator keluaran digital (DO), sedangkan pin Analog Out (AO) mengeluarkan sinyal mentah.
- Pada firmware versi 3.1.1, algoritma pendeteksi kehadiran sensor membaca amplitudo jendela cuplik 50 ms (`peak-to-peak = high - low`) dan memvalidasi bahwa sinyal berada dalam rentang wajar (tidak macet mengambang atau korslet penuh ke rel 0V atau 3.3V).

---

## 4. Arsitektur Perangkat Lunak dan Alur Data

### 4.1 Alur Telemetri dan Kontrol
1. **Siklus Cuplik ESP32**: Setiap 5 detik (parameter `iv`), firmware mengukur suhu dan kelembapan dari 3 DHT11 serta menghitung puncak amplitudo kebisingan dari 3 KY-037 selama durasi cuplik 50 ms.
2. **Pengiriman HTTP POST**: ESP32 menyusun paket JSON dan mengirimkan `POST /device/readings` ke server dengan menyertakan status aktuator yang sedang aktif (`applied`).
3. **Penyimpanan dan Evaluasi Server**:
   - Server memverifikasi bearer token.
   - Server menyimpan data pembacaan ke tabel `readings`.
   - Modul `internal/decision` membandingkan pembacaan sensor dengan pengaturan ambang batas tersimpan:
     - **Kondisi Panas**: Suhu ruangan $> B_n$ (di mana $B_n$ adalah ambang batas suhu per ruang, default 30.0 °C). Kipas menyala jika ada ruangan panas, dan baru mati jika suhu seluruh ruangan turun di bawah $B_n - \text{Hys}$ (histeresis).
     - **Kondisi Bising**: Amplitudo kebisingan $> B_s$ (ambang batas kebisingan, default 1000 ADC span).
     - **Aktivasi Buzzer**: Jika alarm diaktifkan di pengaturan (`al=true` untuk suhu atau `na=true` untuk kebisingan), buzzer pada ruang terkait dinyalakan.
4. **Respons Perintah**: Server merespons HTTP 200 dengan payload status instruksi (`desired`) dan daftar perintah antrean (`commands`).
5. **Penerapan di Board**: ESP32 menerima payload respons dan langsung menerapkan sinyal digital HIGH/LOW ke relay kipas dan basis transistor buzzer.

### 4.2 Struktur Tabel Database SQLite

- `devices`: Menyimpan data node (`id`, `name`, `token_hash`, `firmware`, `ip`, `mode`, `last_seen_at`).
- `readings`: Menyimpan seluruh rekaman telemetri berkala (`id`, `device_id`, `ts`, `sensors`). Dilengkapi indeks `readings_device_ts`.
- `actuators`: Menyimpan status kendali terkini (`device_id`, `fan`, `buzzer`, `updated_at`).
- `settings`: Menyimpan ambang batas operasional (`b1`, `b2`, `b3`, `hys`, `bs`, `iv`, `al`, `na`, `fm`, `bm`).

---

## 5. Implementasi Antarmuka Pengguna (Web Dashboard)

Dasbor web LibraSense berfungsi sebagai pusat visualisasi dan kendali operator perpustakaan:
1. **Kartu Telemetri Ruangan**: Menampilkan nilai suhu, kelembapan, dan tingkat kebisingan untuk Ruang Baca, Ruang Rak 1, dan Ruang Rak 2. Menggunakan kartu status visual dengan indikator normal, peringatan panas, dan peringatan bising.
2. **Denah Vektor Maket Perpustakaan (SVG Interaktif)**:
   - Merefleksikan tata letak maket kardus fisik: dinding luar cokelat bergelombang, sayap depan bergaris biru, pintu masuk utama, rak buku berderet, dan meja-kursi belajar.
   - Setiap ruangan dilengkapi chip HUD melayang (*floating HUD chip*) yang menampilkan telemetri langsung secara reaktif melalui binding data Alpine.js.
   - Lantai denah ruangan memiliki transisi warna otomatis (`hot` menjadi kemerahan, `noisy` memiliki border oranye tegas) saat nilai sensor melewati batas bahaya.
3. **Grafik Riwayat Suhu**: Vektor SVG dinamis yang merender tren suhu 120 sampel terakhir per ruangan secara langsung tanpa dependensi pustaka visualisasi eksternal (seperti Chart.js atau D3).
4. **Panel Kontrol Pengaturan**: Formulir interaktif untuk mengubah ambang suhu per ruang, nilai histeresis, batas kebisingan, sakelar alarm, serta tombol uji manual aktuator (`test1`, `test2`, `test3`, `reset`, `restart`).

---

## 6. Infrastruktur dan Deployment Produksi

- **Target Lingkungan**: Mesin VPS Ubuntu 24.04 LTS (Resource-constrained VPS, 2 vCPU, RAM 2 GB, shared host dengan tenant aplikasi lain).
- **Container Engine**: Podman 4.9.3 (rootful) dengan systemd service unit `librasense.service`.
- **Topologi Jaringan Edge**:
  - Binary di dalam kontainer hanya mendengarkan pada antarmuka lokal host `127.0.0.1:8097`.
  - Akses publik keluar dialihkan melalui daemon Cloudflare Tunnel (`cloudflared`).
  - Dua hostname publik diarahkan ke service yang sama:
    1. `librasense.<domain>`: Diproteksi Cloudflare Zero Trust Access (khusus pengguna dasbor).
    2. `iot.<domain>`: Tanpa login web Cloudflare Access, digunakan khusus jalur telemetri ESP32 dengan pengamanan bearer token mandiri.
- **Prosedur Rilis Otomatis**:
  - `deploy/build.ps1`: Kompilasi silang (*cross-compile*) Go dari lingkungan pengembangan ke binary target Linux `x86_64` tanpa memerlukan toolchain Go di server VPS.
  - `deploy/deploy.ps1 -Remote`: Mengirimkan konteks build, membangun image lokal Podman di host VPS, merestart service systemd, dan memvalidasi endpoint `/health` secara otomatis.

---

## 7. Kronologi Masalah Teknis dan Solusi Rekayasa

1. **Sensor Suara KY-037 Terbaca Rusak / Tidak Hadir (`ok=false`)**:
   - *Masalah*: Nilai kebisingan untuk Ruang Baca dan Ruang Rak 2 sempat menampilkan tanda `--` di panel web.
   - *Penyebab*: Modul KY-037 memiliki karakteristik offset DC titik diam yang bervariasi antar pabrikan. Firmware versi awal memeriksa ambang rata-rata statis `mean > 100`. Modul dengan baseline DC rendah jatuh di bawah nilai 100 sehingga salah terdeteksi sebagai sensor tidak terpasang.
   - *Solusi*: Logika deteksi kehadiran sensor disempurnakan untuk memvalidasi batas sinyal rel daya (`count > 0 && high > 0 && low < 4095`), bukan rata-rata DC statis. Pembaruan firmware 3.1.1 menyelesaikan masalah ini dan seluruh sensor terbaca `ok=true`.
2. **Resolusi dan Karakteristik Sensor DHT11**:
   - *Masalah*: Grafik garis suhu tampak mendatar dalam beberapa menit pengamatan awal.
   - *Analisis*: Sensor DHT11 memiliki resolusi pembacaan diskret 1.0 °C. Pada kondisi ruangan tertutup yang suhunya stabil, nilai suhu tidak mengalami fluktuasi cepat dalam rentang waktu singkat 10 menit. Hal ini merupakan karakteristik intrinsik sensor dan bukan indikasi anomali sistem atau data beku (*stale*).

---

## 8. Ringkasan Parameter Konfigurasi

| Parameter | Tipe | Rentang Nilai | Default | Deskripsi Fungsional |
|---|:---:|:---:|:---:|---|
| `b1` | float | 10.0 – 50.0 °C | 30.0 | Ambang batas suhu Ruang Baca |
| `b2` | float | 10.0 – 50.0 °C | 30.0 | Ambang batas suhu Ruang Rak 1 |
| `b3` | float | 10.0 – 50.0 °C | 30.0 | Ambang batas suhu Ruang Rak 2 |
| `hys` | float | 0.0 – 10.0 °C | 1.0 | Rentang histeresis suhu untuk mematikan kipas |
| `bs` | integer | 0 – 4095 ADC | 1000 | Ambang batas amplitudo kebisingan KY-037 |
| `iv` | integer | 1000 – 60000 ms | 5000 | Interval pengiriman data telemetri dari node |
| `al` | boolean | true / false | true | Sakelar izin aktivasi buzzer jika terdeteksi suhu panas |
| `na` | boolean | true / false | false | Sakelar izin aktivasi buzzer jika terdeteksi kebisingan |
| `fm` | integer | 0 (Auto), 1 (Manual) | 0 | Mode kontrol kipas pendingin |
| `bm` | integer | Bitmask (0–7) | 7 | Mode kontrol manual buzzer tiap ruangan |
