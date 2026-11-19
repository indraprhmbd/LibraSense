# Konvensi docs

Folder ini menyimpan catatan teknis yang bisa dibaca ulang oleh manusia maupun agent. Tujuannya menjaga keputusan tetap terlacak setelah percakapan hilang.

## Penamaan

`NNN-slug-ddmmyyyy.md` dengan bulan tiga huruf, contoh `001-go-architecture-19-nov-2026.md`.

Nomor urut tidak pernah dipakai ulang. Dokumen yang salah tidak dihapus, statusnya diubah menjadi `superseded` dan diganti dokumen baru.

## Header wajib

```
Status: draft | in-progress | done | superseded
Tanggal: 19 Nov 2026
Pemilik: ...
Terkait: docs/00X-...
```

## Isi

- Tulis bahasa Indonesia, istilah teknis tetap bahasa aslinya.
- Tanpa emoji, tanpa em dash dan en dash.
- Ringkas dan spesifik. Jangan mengarang angka, pin, atau hasil pengujian.
- Setiap klaim teknis harus bisa ditelusuri ke file atau perintah.
- Catat keputusan beserta alasannya, bukan hanya hasil akhir.
- Bagian terbuka ditulis eksplisit, jangan disembunyikan.

## Journal

`journal.json` adalah indeks tambah saja. Setiap dokumen baru menambah satu entri:

```
{ "id": "003", "date": "2026-11-20", "file": "003-....md", "title": "...", "status": "done", "summary": "..." }
```

Jangan mengubah entri lama kecuali statusnya berubah.

## Template

Salin `000-template-name-19-nov-2026.md`.
