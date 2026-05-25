# Aether CBT — Quick Start (Developer)

This guide gets the entire multi-tenant CBT platform running locally in under 5 minutes.

## Prerequisites
- Go 1.22+
- Node.js 18+ (for frontend)
- Git

## 1. One-time Setup

```bash
cd D:\Users\Saroel-H\Projects\cbt_sekolah
```

## 2. Start Everything (Recommended)

From the project root:

```bash
npm run dev
```

This will automatically start **both** backend (Go) and frontend (SvelteKit) together using `concurrently`.

On first run the server will automatically:
- Create `data/cbt_aether.db`
- Run all 11 migrations
- Create default tenant + admin user

**Default Admin Login**
- Username: `admin`
- Password: `admin123`

## 3. Seed Sample Data (Optional but Recommended)

In another terminal (while `npm run dev` is running):

```bash
npm run seed
# or
go run cmd/seed/main.go
```

This adds:
- 3 classes
- 4 subjects
- 2 exam rooms
- 8 students
- Room supervisors

**Sample Student Credentials**
- No. ID: `2024001`
- Password: `siswa123`
- Exam Token: `ujian2026`

**Room Supervisor**
- `ruang_a` / `ruang123`

## 4. Start the Frontend

```bash
cd web
npm install
npm run dev
```

Frontend will be available at **http://localhost:5173**

## 5. Login Flows

### Admin
1. Open http://localhost:5173/admin
2. Login with `admin` / `admin123`
3. You will see live counts from the database

### Student
1. Open http://localhost:5173/student/login
2. Use: `2024001` + `siswa123` + `ujian2026`
3. You will be taken to the exam screen

### Supervisor (placeholder)
http://localhost:5173/supervisor/login

## Useful Commands

| Command           | Description                        |
|-------------------|------------------------------------|
| `make run`        | Start backend                      |
| `make seed`       | Seed sample data                   |
| `make clean`      | Delete database + rebuild          |
| `go run cmd/server/main.go` | Direct backend start     |

## API Testing (with curl or Postman)

All requests need header:
```
X-Tenant-ID: 1
Authorization: Bearer <token>
```

Login first to get token.

## Environment Variables (PENTING!)

Beberapa variabel lingkungan sekarang **WAJIB** diisi untuk menjalankan aplikasi dengan aman:

| Variable                  | Wajib?     | Keterangan |
|---------------------------|------------|----------|
| `JWT_SECRET`              | **Ya**     | Rahasia untuk JWT. Harus diisi dengan string panjang & acak. Aplikasi akan **gagal start** jika kosong. |
| `CORS_ALLOWED_ORIGINS`    | Direkomendasikan di production | Daftar origin yang diizinkan (pisah dengan koma). Contoh: `https://cbt.sekolah.sch.id` |
| `PORT`                    | Opsional   | Default: 3000 |
| `DATABASE_URL`            | Opsional   | Default: `data/cbt_aether.db` |

**Peringatan Keamanan**: Jangan pernah menggunakan secret lemah atau default di lingkungan produksi.

## Next Steps

- Build more advanced admin forms
- Add real-time SSE for supervisor monitoring
- Integrate actual iSpring content in `data/soal/`
- Add Excel import

The foundation is now complete and ready for real school deployment development.
