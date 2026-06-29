## 📘 Cara Menggunakan MangaDex Downloader

Tool ini dibuat sepenuhnya oleh AI

Wajib lapor jika ada bug atau error

### 1. Jalankan Program
Buka **Command Prompt** atau **PowerShell** (terminal), lalu:

Windows:
```
cd d:\mangadownv2
.\mangadown.exe
```

Linux/termux:
```
cd d:/mangadownv2
./mangadown.exe
```

### 2. Cari Manga
Program akan meminta judul manga:
```
🔍 Search manga: one piece
```
Ketik judul lalu tekan **Enter**.

### 3. Pilih Manga
Hasil pencarian akan tampil:

```
📚 Results:
  [1] One Piece Academy
  [2] One Piece
  [3] One Piece Party
  ...

Select manga [1-10]:
```
Ketik nomor pilihannya lalu **Enter**.

### 4. Pilih Bahasa
Program akan scan semua bahasa yang tersedia:

```
🌐 Available languages:
  [1] English (en)
  [2] Indonesian (id)
  [3] Thai (th)
  ...

Select language [1-15]:
```
Ketik nomor bahasa yang kamu mau lalu **Enter**.

### 5: Atur Concurrent

```
⚡ Concurrent downloads (default 5, press Enter for default):
```

maksudnya **berapa chapter yang di-download bersamaan**.

| Angka | Cocok untuk |
|-------|-------------|
| `1`  | Koneksi lemot / tidak stabil |
| `5`  | Koneksi standar / default |
| `10` | Koneksi kencang / WiFi stabil |
| `20` | Koneksi super kencang |

> **⚠️ Tips**: Jika koneksi kamu lagi lemot, pakai `3` atau `5` aja biar nggak timeout. Kalau kenceng, `10` juga oke.

Kalau bingung, tinggal Enter aja (pakai default 5).

### 6. Proses Download

```
⬇️  Starting download...
    [1/49] Ch. 1
    [2/49] Ch. 2
    [3/49] Ch. 3
    ...
```

Progress bar bakal muncul buat tiap chapter:

```
Ch. 1  ████████████████░░░░░░░  68%  (38/55)
```


Setiap chapter disimpan di folder:
```
JUDUL_MANGA/
├── Vol. 1 - Ch. 1
│   ├── 001.png
│   ├── 002.png
│   └── ...
├── Vol. 1 - Ch. 2
└── ...
```

### 7. Log
Semua aktivitas dicatat di folder `logs/`:
```
logs/mangadown_20260628_220603.log
```

### Tips
- **Pakai Input Otomatis (pipe):**
  ```
  "naruto`n1`n5`n" | .\mangadown.exe
  ```
  Artinya: cari "naruto" → pilih nomor 1 → pilih bahasa nomor 5 → download semua

- **Jika download gagal** di tengah, tinggal jalankan ulang. File yang sudah terdownload akan di-skip (resume support).

- **Pastikan koneksi internet stabil**, MangaDex butuh koneksi untuk ambil gambar di setiap chapter.
