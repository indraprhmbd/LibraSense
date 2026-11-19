Status: in-progress
Tanggal: 19 Nov 2026
Pemilik: tim LibraSense
Terkait: 001-go-architecture-19-nov-2026.md

# Firmware kirim data, tanpa logika keputusan

## Konteks

Firmware lama menjalankan web server sendiri dan memutuskan kipas, buzzer, serta alarm secara lokal. Arsitektur baru memindahkan semua keputusan ke server Go, sehingga firmware hanya membaca sensor, mengirim data, dan menjalankan state yang dikembalikan server. Firmware lama juga memakai GPIO12 untuk buzzer, pin strapping MTDI yang bisa menghalangi boot.

Sketsa lama sempat hilang bersama folder proyek. Salinan ditemukan di `dump/sketch_nov18a/sketch_nov18a.ino` dan dipakai sebagai rujukan pin serta perilaku sensor.

## Daftar modul

| Komponen | Jumlah | Catatan |
|---|---|---|
| ESP32 Development Board | 1 | controller |
| ESP32 Expansion Board | 1 | ekspansi pin |
| DHT11 | 3 | suhu dan kelembapan per ruang |
| KY-037 | 3 | keluaran analog, bukan dB(A) |
| Relay 1 kanal | 1 | memutus dan menyambung kipas 12V |
| DC fan 12V | 1 | beban relay |
| LCD 16x2 I2C | 1 | informasi lokal |
| Buzzer | 3 | indikator per ruang |

Kode firmware sekarang memakai `#define RACK_COUNT 3` sehingga ketiga ruang aktif: ruang baca dan dua ruang rak. Protokol tidak berubah karena kontrak sudah membawa tiga slot buzzer. Rangkaian lengkap, catu daya, dan peringatan tegangan ada di 005.

## Pin map

| Fungsi | GPIO | Alasan |
|---|---|---|
| DHT11 ruang baca, rak 1, rak 2 | 23, 19, 13 | pin aman, tidak strapping |
| KY-037 AO ruang baca, rak 1, rak 2 | 34, 35, 32 | ADC1, 34 dan 35 input-only, cocok untuk analog |
| Buzzer ruang baca, rak 1, rak 2 | 26, 25, 33 | 33 menggantikan 12 yang strapping |
| Relay kipas | 27 | aktif rendah, default mati saat boot |
| LCD SDA, SCL | 21, 22 | I2C bawaan |

GPIO12 dihindari karena strapping MTDI. GPIO6 sampai 11 dipakai flash. GPIO0, 2, 12, 15 strapping. GPIO1 dan 3 UART0. GPIO34 sampai 39 input-only, jadi hanya untuk sensor analog.

## Kontrak perangkat

Kiriman `POST /device/readings`:

```
{
  "firmware": "3.1.0",
  "ip": "192.168.18.44",
  "mode": "WiFi",
  "ts": 1763520000000,
  "applied": {
    "fan": true,
    "relay": true,
    "buzzer": [true, false, false],
    "uptime": 320
  },
  "sensors": [
    { "id": "dht1", "type": "temperature", "value": 34.5, "ok": true, "unit": "C", "rack": 1 },
    { "id": "hum1", "type": "humidity", "value": 61, "ok": true, "unit": "%", "rack": 1 },
    { "id": "sound1", "type": "noise", "value": 1200, "ok": true, "unit": "adc", "rack": 1 }
  ]
}
```

Balasan berisi `state`, `status`, dan `commands`:

```
{
  "ok": true,
  "state": { "fan": true, "buzzer": [true, false, false], "poll_ms": 5000 },
  "status": { "hot": [1], "noisy": [] },
  "commands": [ { "id": 4, "command": "test2", "payload": null } ]
}
```

Firmware tidak menyimpan ambang. `poll_ms` berasal dari pengaturan server, dibatasi 5000 sampai 60000 ms.

Perintah yang dikenali: `restart`, `reset`, `test1`, `test2`, `test3`. `reset` menghapus Preferences dan memulai ulang. `test1..3` menyalakan buzzer 400 ms tanpa mengubah state.

## Perilaku saat jaringan putus

Perangkat tetap membaca sensor tiap interval dan mempertahankan state aktuator terakhir yang diperintahkan server. Tidak ada evaluasi lokal, sesuai keputusan arsitektur. Konsekuensinya, kipas dan buzzer tidak bereaksi terhadap suhu baru selama server tidak terjangkau. Ini harus ditulis di laporan sebagai batasan, bukan disembunyikan.

WiFi disimpan di Preferences. Bila gagal tersambung 15 detik saat boot, perangkat membuka AP `Perpustakaan-ESP32` dengan portal untuk mengisi SSID dan sandi. Kredensial API ada di `secrets.h`, tidak bisa diubah dari portal.

## Keamanan

- Token perangkat dikirim sebagai bearer, bukan di query string.
- Firmware memakai `TLS_INSECURE true` karena sertifikat Cloudflare tidak dipin. Ini melemahkan verifikasi sertifikat dan harus diganti pinning atau CA bundle bila dipakai di luar prototipe.
- `secrets.h` masuk `.gitignore`. Contoh ada di `secrets.example.h`.

## Verifikasi

- Firmware dikompilasi dengan arduino-cli 1.6.0, core esp32 3.3.11, library DHT sensor 1.4.7, fqbn `esp32:esp32:esp32`. Berhasil tanpa error, sketch 1095804 byte (83 persen), variabel global 50648 byte (15 persen). Rincian di 004.
- Kontrak request dan response sudah diuji dari sisi server lewat smoke test di dokumen 001 dan test `internal/httpapi`.

## Terbuka

- Unggah ke board dan uji aktuator belum dilakukan.
- Kalibrasi KY-037: keluaran analog bukan dB(A). Ambang `bs` masih nilai ADC mentah.
- Perilaku `RELAY_ACTIVE_LOW` harus dicek pada modul relay yang dipakai. Bila modul aktif tinggi, ubah konstanta dan default pin.
- Belum ada watchdog perangkat. Backoff kiriman sudah ada, lihat 004.

## Rujukan

- `esp32/librasense_node/librasense_node.ino`
- `esp32/librasense_node/secrets.example.h`
- `dump/sketch_nov18a/sketch_nov18a.ino`
