Status: done
Tanggal: 20 Nov 2026
Pemilik: tim LibraSense
Terkait: 008-denah-maket-perpustakaan-19-nov-2026.md, 007-ky037-presence-check-19-nov-2026.md

# Jejak sinyal kebisingan pada denah dan ambang rendah

## Konteks

Panel menampilkan kebisingan hanya sebagai satu angka per ruang. Pada presentasi, angka yang bergerak tiap 5 detik sulit dibaca sebagai aktivitas sensor. Nilai juga sering terlihat datar karena ambang bawaan `bs` sebesar 1000 jauh di atas puncak KY-037 yang hanya 20 sampai 60 pada kondisi tenang.

## Perubahan

1. **Jejak sinyal pada denah (`web/index.html`, `web/app.js`)**
   - Tiap kartu HUD di denah kini punya strip 155x14 satuan SVG di bawah baris kebisingan.
   - `store.pushNoise` menambah satu sampel per ruang pada tiap pembacaan baru, dengan batas 40 sampel terakhir (sekitar 3 menit pada interval 5 detik).
   - `store.seedNoise` mengisi buffer sekali dari `/api/history` saat panel dimuat, sehingga garis tidak kosong pada pembukaan pertama.
   - `noisePoints(rack)` mengubah buffer menjadi polyline. Skala normalisasi memakai nilai puncak buffer dengan lantai 60, jadi garis tetap terbaca walau ruangan sunyi.
   - Sampel dilewati bila sensor tidak hadir (`ok=false`), supaya garis tidak jatuh ke nol saat sensor gagal.
2. **Penanda status (`web/app.css`)**
   - Garis berwarna teal saat normal, menjadi warna peringatan saat `noisy`.
   - Titik ujung kanan berdenyut halus saat berisik, sebagai penanda aktivitas.
3. **Ambang kebisingan bawaan (`internal/settings/settings.go`)**
   - Nilai bawaan `bs` turun dari 1000 menjadi 20 agar peristiwa kebisingan mudah terpicu pada prototipe dan saat demonstrasi.

## Catatan

Ambang tersimpan pada database yang sudah berjalan tidak berubah otomatis. Nilai lama tetap dipakai sampai operator menekan Simpan di tab Pengaturan.
