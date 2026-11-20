Status: done
Tanggal: 20 Nov 2026
Pemilik: tim LibraSense
Terkait: 009-jejak-sinyal-kebisingan-20-nov-2026.md

# Gelombang dan riak kebisingan pada denah

## Konteks

Entri 009 menampilkan kebisingan sebagai garis jejak datar di dalam kartu HUD. Garis itu terbaca seperti grafik riwayat, bukan seperti sinyal suara. Pada presentasi, gelombang yang bergerak lebih cepat dikenali sebagai aktivitas sensor.

## Perubahan

1. **Strip gelombang pada HUD (`web/index.html`, `web/app.css`)**
   - Polyline statis diganti satu gelombang sinus tertutup berukuran 200 x 14 satuan yang bergeser mendatar terus-menerus.
   - Gelombang dipotong memakai `clipPath` per kartu (`sigClip1`, `sigClip2`, `sigClip3`) agar tidak keluar dari kotak strip.
   - Amplitudo diatur properti CSS `--amp` yang diisi dari `noiseAmp(rack)` di `web/app.js` dengan skala `scaleY`, sehingga nilai KY-037 yang kecil tetap terlihat.
   - Saat ruangan berisik, warna garis berubah menjadi `--warn` dan tempo geser naik dari 3,4 detik menjadi 1,1 detik per siklus.
2. **Riak pada lantai denah (`web/index.html`, `web/app.css`)**
   - Tiap ruang punya tiga lingkaran konsentris di titik sensor suara yang tumbuh keluar dan memudar, menyerupai riak air.
   - Riak dipotong memakai `clipPath` sesuai persegi ruang (`rippleRoom1` sampai `rippleRoom3`), jadi gelombang berhenti di dinding.
   - Saat berisik, riak berwarna `--warn`, lebih tebal, dan bertempo 2,4 detik per siklus.
   - Titik pusat: Ruang Baca (300, 350), Ruang Buku 1 (462, 250), Ruang Buku 2 (462, 545).
3. **Penyederhanaan data (`web/app.js`)**
   - Buffer `noiseTrace`, `pushNoise`, `seedNoise`, `noisePoints`, dan `noiseLevel` dihapus.
   - Penggantinya `noiseAmp(rack)`, yang menghitung amplitudo 0 sampai 1 dari nilai sensor terakhir dengan batas bawah 0,18 dan pembagi 120.
4. **Aksesibilitas (`web/app.css`)**
   - Blok `prefers-reduced-motion` mematikan animasi gelombang dan riak, lalu menampilkan riak diam dengan opasitas 0,22.

## Catatan

Animasi hanya berjalan di peramban tanpa preferensi gerak berkurang. Tangkapan layar otomatis pada lingkungan uji menampilkan versi diam.
