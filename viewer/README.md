# Viewer — Frontend

React + TypeScript + Tailwind CSS. Chạy từ thư mục này:

```bash
npm ci
npm run dev
npm run lint
npm run typecheck
npm test
npm run build
```

WAS cần chạy ở localhost:8080 hoặc địa chỉ `API_PROXY_TARGET`. Vite chạy port 5173 và proxy cùng origin cho API/health/OpenAPI.

Dev/build tự chuẩn bị Swagger assets và sinh `src/api.generated.ts` từ [contract WAS](../was/api/openapi.yaml). Viewer build cần contract bên cạnh, không cần mã Go hay tài liệu thiết kế. Vì thế giữ layout workspace khi checkout.

E2E toàn hệ thống chạy từ root bằng `make test-e2e`; runner tự khởi chạy WAS test và Vite, dùng PostgreSQL thật. Xem [hướng dẫn workspace](../README.md). Dockerfile của Viewer dùng context root để copy contract; Docker đang tạm hoãn kiểm chứng.
