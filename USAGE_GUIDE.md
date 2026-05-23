# Aether CBT - Panduan Penggunaan

## 1. Menjalankan Aplikasi

```bash
go run cmd/server/main.go
```

Server akan berjalan di: `http://localhost:3000`

---

## 2. Login Admin

**Endpoint:**
```
POST /api/auth/login
```

**Body:**
```json
{
  "username": "admin",
  "password": "admin123"
}
```

**Catatan:** User admin belum dibuat secara otomatis. Jalankan script `cmd/createadmin` untuk membuatnya.

---

## 3. Endpoint Penting

### Public Endpoints
- `GET /api/health` - Health check
- `POST /api/auth/login` - Login admin/supervisor
- `POST /api/auth/student-login` - Login peserta
- `POST /api/ispring/webhook` - Menerima hasil dari iSpring

### Protected Endpoints (butuh token)
- `GET /api/students` - Daftar peserta
- `GET /api/classes` - Daftar kelas
- `GET /api/mapel` - Daftar mata pelajaran
- `GET /api/rooms` - Daftar ruangan
- `GET /api/users` - Daftar user

---

## 4. Struktur Multi-Tenant

Setiap request dilindungi oleh `tenant_id`. Saat ini default menggunakan tenant ID = 1.

---

## 5. Status Proyek

- ✅ Backend berfungsi
- ✅ Multi-tenant architecture
- ✅ Authentication (JWT)
- ✅ iSpring integration
- ⚠️ Frontend masih dalam tahap awal
- ⚠️ Belum ada data contoh

---

**Aether CBT siap untuk dikembangkan lebih lanjut.**
