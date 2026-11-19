Status: done
Tanggal: 19 Nov 2026
Pemilik: tim LibraSense
Terkait: 005-rangkaian-3-ruang-19-nov-2026.md

# Panel tidak pernah inisialisasi: urutan Alpine dan app.js

## Gejala

Panel tampil sebagai header dan footer saja. Tidak ada kartu ruang, tidak ada status perangkat, tidak ada grafik, dan tidak ada permintaan ke `/api/*`. Console browser penuh pesan error dan peringatan. Gejala sama pada `librasense.<domain>` setelah login Access maupun pada salinan lokal.

## Penyebab

`web/index.html` memuat dua skrip dengan `defer`:

```html
<script defer src="/vendor/alpine.min.js"></script>
<script defer src="/app.js"></script>
```

Alpine 3.14.8 mengakhiri berkasnya dengan `queueMicrotask(() => { Alpine.start() })`. Skrip `alpine.min.js` dieksekusi lebih dulu karena urutan `defer`, lalu microtask langsung menjalankan `Alpine.start()`. Pada titik itu `/app.js` belum dieksekusi, sehingga `panelApp` masih `undefined`.

Akibatnya Alpine mengevaluasi `x-data="panelApp()"` dan gagal, menandai pohon sudah diproses, lalu tidak mencoba lagi setelah `app.js` mendefinisikan `panelApp`. Tidak ada permintaan API karena `refresh()` tidak pernah dipanggil.

Bukti dari inspeksi halaman:

| Cek | Sebelum perbaikan |
|---|---|
| `typeof window.panelApp` | `function` (didefinisikan, tetapi terlambat) |
| `performance.getEntriesByType('resource')` | hanya `app.css`, `alpine.min.js`, `app.js`, `favicon.ico` |
| `/api/overview` | tidak pernah diminta |
| Isi `document.body.innerText` | hanya judul, navigasi, dan footer |
| `Alpine.$data(document.body).online` | bukan fungsi |

## Perbaikan

Urutan skrip dibalik supaya `panelApp` sudah ada sebelum Alpine memulai:

```html
<script defer src="/app.js"></script>
<script defer src="/vendor/alpine.min.js"></script>
```

`app.js` tidak menyentuh objek `Alpine` di tingkat atas, hanya mendefinisikan `api`, `store`, dan `panelApp`, sehingga aman dijalankan lebih dulu. Tidak ada perubahan pada `app.js`, `app.css`, atau kontrak server.

## Verifikasi

Uji lokal dengan server `ACCESS_DEV_BYPASS=true`, data tiga ruang disemai lewat `/device/readings`.

| Cek | Sesudah perbaikan |
|---|---|
| `typeof window.panelApp` | `function` |
| `/api/overview` dan `/api/history` | diminta saat muat |
| Kartu ruang | `Ruang Baca`, `Ruang Rak 1`, `Ruang Rak 2` |
| Baris sensor | 13 baris terisi |
| Grafik SVG | ada |
| `x-cloak` tersisa | 0 |
| `console.error` dan `console.warn` saat init ulang | tidak ada |
| Status perangkat | `Perangkat luring` pada data uji yang sudah lama |

Panel di redeploy 19 Nov 2026 setelah perubahan ini. Pada hostname perangkat `iot.<domain>`, panel sekarang berinisialisasi dan menampilkan banner 401 dari `/api/overview`, yang memang benar karena `/api/*` wajib Access JWT.

## Catatan

Panel web harus dibuka di `https://librasense.<domain>` dan login Cloudflare Access. Membuka `iot.<domain>` hanya menampilkan panel tanpa data karena hostname itu tidak punya Access application.
