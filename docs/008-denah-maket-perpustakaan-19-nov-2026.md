Status: done
Tanggal: 19 Nov 2026
Pemilik: tim LibraSense
Terkait: 005-rangkaian-3-ruang-19-nov-2026.md, 006-panel-alpine-start-19-nov-2026.md, 007-ky037-presence-check-19-nov-2026.md

# Denah visual maket perpustakaan pada panel

## Konteks

Sebelumnya slot denah pada panel web (`.floorplan-slot`) hanya berisi teks placeholder "Denah belum tersedia". Maket fisik perpustakaan telah dibuat menggunakan kardus dengan tata ruang 3 ruangan:
1. Ruang Utama (Perpustakaan) di sisi kiri, dilengkapi dua rak buku pada dinding kiri, meja dan dua kursi belajar di tengah bawah, serta label nama ruang.
2. Ruang Buku 1 di sisi kanan atas, dilengkapi rak buku di dinding kanan dan label ruang.
3. Ruang Buku 2 di sisi kanan bawah, dilengkapi rak buku di dinding kanan dan label ruang.
4. Pintu Masuk di sisi depan tengah, diapit dinding depan kardus dengan aksen dua garis biru pada masing-masing sayap.

## Perubahan

1. **Denah Vektor SVG (`web/index.html`)**:
   - Dibuat representasi grafis SVG presisi tinggi yang mereplikasi bentuk maket kardus fisik: warna kraft kardus, lipatan tutup kotak luar (flaps), tepi bergelombang (corrugated rim), fasad depan dengan garis biru, pintu masuk, furnitur rak buku dengan buku warna-warni, serta meja dan kursi.
   - Lorong pintu antar-ruangan dibuat terbuka menghubungkan Ruang Utama dengan kedua Ruang Buku.
2. **Overlay Telemetri Interaktif**:
   - Masing-masing dari ketiga ruangan dilengkapi kartu HUD telemetri langsung (*live HUD chip*):
     - Titik status indikator aktif (hijau untuk normal, oranye untuk panas/panik, abu-abu saat luring).
     - Pembacaan langsung suhu (°C), kelembapan (%), dan kebisingan.
     - Penanda peringatan teks `(berisik)` otomatis muncul saat sensor kebisingan melewati batas.
   - Lantai ruangan memiliki kelas `:class="{ hot: rack(n).hot, noisy: rack(n).noisy }"` yang memberi aksen warna latar saat suhu panas atau berisik terdeteksi.
3. **Penyelarasan Kode Alpine (`web/app.js` dan `web/app.css`)**:
   - Ditambahkan pembantu `rack(num)` pada store Alpine untuk mempermudah akses status reaktif per ruangan dari SVG.
   - Pengaturan CSS transisi halus untuk pergantian status dan tata letak responsif denah.

## Verifikasi

1. Uji visual dilakukan langsung di browser lokal menggunakan `chrome-devtools`.
2. Denah merender penuh di dalam panel dasbor, responsif terhadap tema terang dan gelap, dengan data telemetri ketiga ruangan terhubung ke state Alpine secara waktu-nyata.
3. Semua verifikasi build dan uji Go berhasil (`go vet ./...`, `go test ./...`, `go build ./cmd/librasense`).
