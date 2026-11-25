# Review Phase 01

Ngày 2026-11-25. Reviewer độc lập: sub-agent `phase01_reviewer`; executor sửa và chạy kiểm chứng lại. Đây là review mã nguồn/thiết kế trong phạm vi phase, không thay thao tác approve artifact.

## Phạm vi

Đã rà plans, requirements/architecture/ERD/state machine/API/events, Go config/database/HTTP/runtime/migration/seed/worker, React client/UI, Compose/Dockerfile/Makefile/scripts, unit/integration/E2E, README và evidence. Không coi module Phase 02+ chưa triển khai là defect.

## Findings đã xử lý

| ID | Mức độ | Vị trí | Vấn đề / sửa | Bằng chứng |
| --- | --- | --- | --- | --- |
| R01 | High | was/migrations/migrate.go, was/internal/config/config.go | Query DB override có thể vượt guard tên *_test. Reject option ghi đè/URL path bất thường và thêm regression test | Go unit/race, smoke guard |
| R02 | Medium | was/api/openapi.yaml | Tạo admin user thiếu password/setup flow. Tách AdminUserCreateInput có password write-only và AdminUserUpdateInput | OpenAPI validate + semantic review |
| R03 | Medium | was/api/openapi.yaml, docs/api-design.md | Refresh cookie thiếu contract header. Bổ sung Set-Cookie, Path/HttpOnly/SameSite/Secure/rotation/clear | Contract review |
| R04 | Medium | viewer/playwright.config.ts, scripts/run-e2e.py, viewer/e2e/foundation.spec.ts | Có thể pass nhầm Vite/backend cũ. Tắt reuse, khởi tạo APP_VERSION riêng và assert từ API thật | E2E pass |
| R05 | Medium | viewer/src/App.tsx, viewer/public/api-docs/index.html | URL directory Swagger trên Vite trả SPA. Đổi link index.html, bundle assets local và tắt remote validator | Chrome E2E mở Swagger thật |
| R06 | Low | .gitignore, .dockerignore | Build metadata TypeScript bị đưa vào danh sách source. Ignore *.tsbuildinfo | Git inventory |

## Kết quả review cuối

- Không còn finding mã nguồn/contract cần sửa được phát hiện trong phạm vi Phase 01.
- OpenAPI có 61 operations/41 schemas, 4 endpoint Phase 01; không unresolved ref, path-param mismatch hoặc operationId trùng.
- Ảnh desktop/mobile không có lỗi bố cục đáng kể; UI dùng cấu hình DB thật, không giả catalog/order.
- Integration PostgreSQL, Go race/vet, frontend checks, Chrome E2E và 9 native smoke checks có evidence trong [progress](PROGRESS.md).

## Giới hạn và kết luận

Compose mới validated cấu hình. Docker daemon permission denied và rootless thiếu system requirements nên không có runtime evidence container. Giữ gate Compose chưa hoàn thành; kết luận là **mã nguồn và native verification hoàn tất, chưa nghiệm thu đầy đủ Phase 01 qua Compose**.
