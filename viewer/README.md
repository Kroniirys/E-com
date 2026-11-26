# Viewer — Frontend

React + TypeScript + Tailwind CSS. Chạy từ thư mục này:

```bash
npm ci
npm run dev
npm run format:check
npm run lint
npm run typecheck
npm test
npm run build
```

WAS cần chạy ở localhost:8080 hoặc địa chỉ `API_PROXY_TARGET`. Vite chạy port 5173 và proxy cùng origin cho API/health/OpenAPI.

Dev/build tự chuẩn bị Swagger assets và sinh `src/api.generated.ts` từ [contract WAS](../was/api/openapi.yaml). Viewer build cần contract bên cạnh, không cần mã Go hay tài liệu thiết kế. Vì thế giữ layout workspace khi checkout.

Các màn hình Phase 02: storefront/detail, đăng ký/đăng nhập, account/profile/address/password và admin users/products/categories/inventory. Token access chỉ giữ trong memory; refresh dùng HttpOnly cookie cùng origin. `npm run format` định dạng mã nguồn; `npm run format:check` chỉ kiểm tra.

E2E toàn hệ thống chạy từ root bằng `make test-e2e`; runner tự khởi chạy WAS test và Vite, dùng PostgreSQL/Redis thật với schema/key prefix riêng. Xem [hướng dẫn workspace](../README.md). Dockerfile của Viewer dùng context root để copy contract; Docker đang tạm hoãn kiểm chứng.

Phase 03 thêm `/#/cart`, `/#/orders` và `/#/admin/orders`: COD/mock, retry/hủy/lifecycle/history. Intent chưa có kết quả được giữ trong sessionStorage theo user; không lưu access token. E2E cô lập từng spec bằng API/schema/Redis prefix riêng để giữ nguyên rate limit thật.

Phase05 thêm /#/notifications. E2E cần TEST_RABBITMQ_URL để chạy worker; Docker browser đặt E2E_EXTERNAL_SERVER và E2E_BASE_URL. Artifact ảnh/log native và Docker tách thư mục.
