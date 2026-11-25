# Tiến độ triển khai

Cập nhật 2026-11-25. Không có ước lượng thời gian.

## Phase 01 — Mã nguồn và kiểm chứng native đã hoàn tất; Docker tạm hoãn

Đã triển khai các công việc trong [plan Phase 01](../plans/01-foundation-and-design-plan.md), có sub-agent `phase01_reviewer` review độc lập và sửa lặp. Không còn finding mã nguồn/contract cần sửa được phát hiện trong vòng review cuối. Theo yêu cầu mới nhất, tạm hoãn kiểm chứng Docker; không ghi Docker pass và không dùng phần hoãn này để chặn việc cơ cấu lại WAS/Viewer.

| Công việc | Kết quả | Đầu ra / evidence |
| --- | --- | --- |
| P01-01 | Đạt | [Requirements](requirements.md), [decision log](architecture-decisions.md) |
| P01-02 | Đạt | [Architecture](architecture.md), [ERD 25 bảng thiết kế](erd.md), [state machine](order-payment-state-machine.md), [events](events.md) |
| P01-03 | Đạt | [API design](api-design.md), [OpenAPI](../was/api/openapi.yaml): 61 operations/41 schemas, 4 endpoint hiện thực |
| P01-04 | Mã nguồn đạt; Compose runtime chưa kiểm chứng | Go API/worker/config/logging/shutdown, React UI thật, Dockerfile/Compose; native smoke/E2E đạt |
| P01-05 | Đạt trên PostgreSQL thật | Migration up/repeat/rollback/re-up, seed lặp/không ghi đè, schema test riêng, reset guard |
| P01-06 | Đạt tài liệu và native clean path; cần bổ sung kết quả Compose khi có quyền | README, progress, known issues, review và logs |

## Kết quả kiểm chứng đã chạy

Môi trường: Go 1.27.4, Node 24.22.3, npm 11.19.1, PostgreSQL 17.12 chạy độc lập trên localhost, Google Chrome 157 qua Playwright 1.65.1. Dependency được khóa trong `was/go.mod`, `was/go.sum` và `viewer/package-lock.json`. Binary cuối build với CGO_ENABLED=0 giống Dockerfile.

| Kiểm tra | Lệnh / phương pháp | Kết quả | Evidence |
| --- | --- | --- | --- |
| Go build | `make GO=/snap/go/current/bin/go build` | Đạt: api/worker/migrate/seed/healthcheck static | [log](evidence/phase-01/build-go.txt) |
| Go unit/race | `go test -race -p 2 ./...` với cache trong /tmp | Đạt: 7 test | [log](evidence/phase-01/go-race.txt) |
| Go vet | `go vet ./...` | Đạt, exit 0 | [log](evidence/phase-01/go-vet.txt) |
| PostgreSQL integration | `go test -tags=integration -count=1 -v ./tests/integration` với TEST_DATABASE_URL *_test | Đạt: 2 test, schema cô lập được dọn | [log](evidence/phase-01/integration.txt) |
| OpenAPI | `npm run validate:api`, generation TypeScript | Đạt, không warning schema | [log](evidence/phase-01/openapi.txt) |
| Reproducible dependency install | `npm ci` | Đạt từ lockfile | [log](evidence/phase-01/npm-ci.txt) |
| Frontend lint/typecheck | `npm run lint`, `npm run typecheck` | Đạt | [lint](evidence/phase-01/frontend-lint.txt), [types](evidence/phase-01/frontend-typecheck.txt) |
| Frontend unit | `npm test` | Đạt: 2 test dữ liệu thật từ client, lỗi và retry | [log](evidence/phase-01/frontend-tests.txt) |
| Frontend build | `npm run build` | Đạt | [log](evidence/phase-01/frontend-build.txt) |
| Chrome E2E | `make test-e2e` với TEST_DATABASE_URL và Chrome sẵn có | Đạt: 1 test API thật → UI, reload, mobile không tràn, Swagger local; xác minh version riêng để không chạy nhầm backend | [log](evidence/phase-01/e2e.txt), [desktop](evidence/phase-01/desktop.png), [mobile](evidence/phase-01/mobile.png) |
| Runtime smoke | Config fail-fast; stop/restart PostgreSQL thật; API/worker SIGTERM | Đạt: 9 checks | [JSON](evidence/phase-01/native-smoke.json), [API log](evidence/phase-01/smoke-api.log), [worker log](evidence/phase-01/smoke-worker.log) |
| Image tags | `docker manifest inspect` cho 5 image dùng trong đường chạy chính | Tồn tại trên registry, không thay bằng chứng runtime | [JSON](evidence/phase-01/image-manifests.json) |
| Compose syntax/wiring | `docker compose --env-file .env.example config --quiet`, mặc định và đủ profiles | Đạt cấu hình, không phải bằng chứng runtime | [log](evidence/phase-01/compose-validation.txt) |
| Compose runtime/clean-start | `docker compose up --build` | Chưa kiểm chứng | Không có quyền daemon; rootless thiếu system requirements |

Unit/component tests được phân biệt với integration thật. Không test integration/E2E nào bị skip rồi coi pass. Native smoke dùng PostgreSQL tạm do phiên triển khai tạo trong /tmp, không dừng database có sẵn của người dùng. Log vet rỗng là output thành công, không phải chưa chạy.

## Review và sửa lặp

Xem [review Phase 01](review-phase-01.md). Đã sửa guard query override DB, admin create/update contract, cookie header contract, E2E chạy nhầm server, Swagger directory routing và ignore metadata build. Vòng cuối không có finding mã nguồn/contract cần sửa.

## Docker — ghi nhận lịch sử và tạm hoãn hiện tại

Docker CLI/Compose có sẵn nhưng kết nối `/var/run/docker.sock` bị permission denied; `sudo -n` yêu cầu mật khẩu. Kiểm tra rootless cho thấy thiếu newuidmap/newgidmap và subordinate UID/GID. Không sửa quyền socket, user hoặc cấu hình hệ thống để vượt giới hạn này.

Khi môi trường có Docker daemon phù hợp, thực hiện đường clean-start trong README trên volume local mới có kiểm soát, xác nhận migration/seed/API/frontend và restart giữ dữ liệu, lưu logs rồi mới đánh dấu tiêu chí Compose hoàn thành. Chưa triển khai Phase 02. Kiểm chứng Docker hiện được tạm hoãn theo yêu cầu của chủ dự án.

## Cập nhật cấu trúc WAS/Viewer

Đã thực hiện yêu cầu tách mã nguồn và review lại bằng sub-agent độc lập. Không còn finding đường dẫn/wiring cần sửa được phát hiện. Evidence Phase 01 trước khi tách giữ nguyên, kiểm bằng SHA-256 của 19 file; kết quả mới lưu riêng.

- Backend: `was/`, gồm Go module, api contract/embed, cmd/internal/migrations/tests/scripts/bin, Makefile và Dockerfile.
- Frontend: `viewer/`, gồm npm project, src/E2E/public/config, Dockerfile/Nginx; generator đọc `was/api/openapi.yaml`.
- Root: docs/plans, environment/Compose/Make điều phối và scripts/run-e2e.py; không còn Go module hoặc source backend tại root.
- E2E runner dùng `was/bin/` và `viewer/`; `E2E_EVIDENCE_DIR` được chuẩn hóa và truyền cho log/ảnh để không ghi đè evidence lịch sử.

| Kiểm tra sau khi tách | Kết quả | Evidence |
| --- | --- | --- |
| `make was-check build` với Go binary hiện có | Đạt 7 unit tests, vet và 5 binary static | [log](evidence/restructure-was-viewer/was-check-build.txt) |
| `make viewer-check` | Đạt OpenAPI/generation/lint/typecheck, 2 unit tests và build | [log](evidence/restructure-was-viewer/viewer-check.txt) |
| `make test-integration` với PostgreSQL test native | Đạt 2 integration tests từ root Makefile qua WAS | [log](evidence/restructure-was-viewer/integration.txt) |
| `make test-e2e` với Chrome/PostgreSQL thật | Đạt 1 E2E: WAS → Viewer → Swagger | [log](evidence/restructure-was-viewer/e2e.txt), [desktop](evidence/restructure-was-viewer/desktop.png), [mobile](evidence/restructure-was-viewer/mobile.png) |
| Cấu trúc, link, lock metadata và evidence lịch sử | Đạt, 19 file lịch sử giữ nguyên hash | [kết quả](evidence/restructure-was-viewer/layout-verification.txt) |
| Docker/Compose | Không chạy theo yêu cầu tạm hoãn | Chỉ sửa đường dẫn trong cấu hình; không tuyên bố đã kiểm chứng Docker ở cấu trúc mới |

Chi tiết review: [WAS/Viewer review](review-was-viewer.md). Các log phía trên trong thư mục phase-01 là bằng chứng lịch sử trước khi tách, không bị viết lại thành log của cấu trúc mới.
