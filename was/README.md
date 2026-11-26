# WAS — Backend

Go module của backend E-commerce Base. Toàn bộ entrypoint, mã nguồn Go, SQL migration, integration tests và binary build nằm trong thư mục này.

```bash
make build
make test
make vet
make run
```

Make nạp `../.env`. Lệnh `go ...` trực tiếp cần environment tương ứng và chạy từ `was/`. Binary xuất ở `bin/`, không commit. Module path giữ `ecommerce.local/base` để không đổi import nghiệp vụ khi di chuyển thư mục.

Contract tại [api/openapi.yaml](api/openapi.yaml), embed vào API qua `api/embed.go`. Viewer chỉ đọc contract để sinh type. Không cần thư mục Markdown `docs/` để build backend.

Phase 02 có module auth/users/catalog/inventory và schema v2. API cần `AUTH_JWT_SECRET` cùng `REDIS_URL`; migrate/seed/worker chỉ đọc cấu hình theo vai trò. Tài khoản demo lấy từ `.env`, tạo bằng script setup tại root.

Integration test dùng `TEST_DATABASE_URL` có database tên `*_test` và `TEST_REDIS_URL`; xem [hướng dẫn workspace](../README.md). Dockerfile backend dùng build context chính thư mục `was/`; Docker đang tạm hoãn kiểm chứng.
