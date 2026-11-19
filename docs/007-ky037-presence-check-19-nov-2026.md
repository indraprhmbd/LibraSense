Status: done
Tanggal: 19 Nov 2026
Pemilik: tim LibraSense
Terkait: 005-rangkaian-3-ruang-19-nov-2026.md, 006-panel-alpine-start-19-nov-2026.md

# Perbaikan deteksi hadir KY-037: Ruang Baca dan Ruang Rak 2 terbaca mati

## Gejala

Di panel, kartu Ruang Baca dan Ruang Rak 2 menampilkan nilai kebisingan `--` dan status sensor dianggap gagal (`ok=false`), padahal Ruang Rak 1 terbaca normal dengan angka sekitar 30 sampai 45.

Pemeriksaan database SQLite pada VPS mengonfirmasi:
```
sound1(rack1)=52 ok=false
sound2(rack2)=38 ok=true
sound3(rack3)=32 ok=false
```
Nilai selisih puncak suara (`high - low`) sebenarnya terukur di ketiga pin (30 sampai 50), tetapi kolom `ok` bernilai `false` untuk pin 34 (ruang baca) dan pin 32 (ruang rak 2).

## Penyebab

Fungsi `readNoise` di firmware memeriksa kehadiran sensor menggunakan rata-rata tegangan DC:
```cpp
const int mean = count ? (int)(sum / count) : 0;
present = mean > 100 && mean < 4000;
```

Modul KY-037 yang beredar memiliki desain keluaran analog (AO) yang berbeda:
- Sebagian modul membawa pembagi tegangan sehingga titik diam (quiescent point) berada di sekitar VCC/2 (sekitar 1600 sampai 2048 pada ADC ESP32). Modul di Ruang Rak 1 memiliki karakteristik ini.
- Sebagian modul lain tidak memiliki offset DC internal atau berada dekat 0 V saat sunyi, sehingga nilai rata-rata sampel berada di bawah 100.
- Akibatnya, `mean > 100` bernilai salah (`false`) untuk Ruang Baca dan Ruang Rak 2, sehingga firmware menandai sensor tidak hadir (`ok=false`).
- Di panel (`web/app.js`), fungsi `sensorValue` memfilter `s.ok === true`. Jika `ok=false`, nilai dianggap `null` dan panel menampilkan `--`.

## Perbaikan

Kriteria kehadiran modul diperbaiki agar tidak bergantung pada titik offset DC tertentu. Modul dianggap hadir selama pin ADC membaca fluktuasi normal dan tidak terikat penuh ke rel daya (tidak mengambang atau korslet ke VCC/GND secara permanen):

```cpp
present = count > 0 && high > 0 && low < 4095;
```

Firmware dinaikkan versinya menjadi `3.1.1`.

## Verifikasi

1. Firmware `3.1.1` dikompilasi dan diunggah ke board via `COM6`.
2. Hash data terverifikasi dan board reset sukses.
3. Sampel database baru diperiksa dari VPS:
```
11:50:05 sound1(rack1)=23 ok=true  sound2(rack2)=21 ok=true  sound3(rack3)=28 ok=true
11:50:00 sound1(rack1)=33 ok=true  sound2(rack2)=25 ok=true  sound3(rack3)=23 ok=true
11:49:55 sound1(rack1)=26 ok=true  sound2(rack2)=25 ok=true  sound3(rack3)=32 ok=true
```
Ketiga sensor suara kini berstatus `ok=true` dan nilai kebisingan tampil di panel untuk semua ruang.
