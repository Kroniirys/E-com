---
title: "Phase 01 — Thiết kế và nền tảng"
artifact: implementation-plan
role: fullstack
status: draft
version: 0.3
owner: "TBD"
approvers: []
last_updated: 2026-11-25
---

# Phase 01 — Thiết kế và nền tảng

## 1. Mục tiêu và đầu vào

Chuyển [plan tổng](00-master-plan.md) thành thiết kế đủ rõ và bộ khung chạy được. Tại thời điểm lập plan chỉ có tài liệu yêu cầu; chưa có mã nguồn kế thừa. Không triển khai đầy đủ nghiệp vụ trong phase này.

## 2. Phạm vi

Phân ranh giới module, ERD, API/event contract và state machine; dựng Go API/React/worker entrypoint, PostgreSQL, migration, config, logging, health và test harness. Chưa tạo handler nghiệp vụ giả hoặc toàn bộ bảng chỉ để đủ sơ đồ.

## 3. Công việc

### P01-01 — Chốt phạm vi và quy tắc nghiệp vụ

- Thực hiện: đối chiếu §4–5 plan tổng; chốt tiền/shipping/SKU/ảnh, stock/hủy đơn/payment retry, coupon/review và quyền customer/admin. Phân biệt yêu cầu nguồn với phương án đề xuất.
- Đầu ra: `docs/requirements.md`, `docs/architecture-decisions.md`; quyết định chưa chốt có tác động và module bị chặn rõ ràng.
- Kiểm tra: mọi nhóm chức năng có quy tắc và phase sở hữu; không dùng giả định chưa xác nhận như fact.
- Phụ thuộc: không.

### P01-02 — Kiến trúc, ERD và state machine

- Thực hiện: thiết kế auth/users/products/categories/cart/orders/inventory/payments/coupons/reviews/notifications; handler → service → repository và composition root đơn giản.
- ERD tổng thể gồm bảng nghiệp vụ nguồn và bảng kỹ thuật cần thiết: outbox, idempotency records, processed events, payment attempts. Ghi FK, unique/check constraint, index, snapshot và history.
- Thiết kế transaction checkout, thứ tự khóa, race payment/cancel/adjustment và hoàn kho một lần. Tách trạng thái order, payment aggregate và từng attempt.
- Đầu ra: `docs/architecture.md`, `docs/erd.md`, `docs/order-payment-state-machine.md`; bảng actor/điều kiện cho mỗi transition và sơ đồ dọc khi dùng Mermaid.
- Kiểm tra: trace được checkout thành công, rollback thiếu hàng, request gửi lại, payment đua với cancel và broker ngừng.
- Phụ thuộc: P01-01.

### P01-03 — API, UI contract và bảo mật phiên đăng nhập

- Thực hiện: chuẩn hóa `/api/v1`, schema, ID, số tiền, timestamp, error envelope, request ID, pagination và allowlist sort/filter.
- Bao phủ endpoint nguồn cùng phần còn thiếu: profile/address/admin users, payment attempts/mock result, notification, dashboard, coupon list và moderation.
- Ghi quyền public/customer/admin, ownership, `Idempotency-Key` và lỗi 400/401/403/404/409/429. Backend quyết định quyền; UI route guard chỉ hỗ trợ trải nghiệm.
- Đề xuất refresh token trong cookie HttpOnly, access token trong bộ nhớ frontend; chốt Secure/SameSite/Origin-CSRF/CORS theo môi trường và ghi decision log. Không lưu refresh token trong localStorage.
- Đầu ra: `was/api/openapi.yaml`, `docs/api-design.md`; mapping màn hình → API và schema TypeScript.
- Kiểm tra: OpenAPI hợp lệ, không endpoint trùng chức năng; các màn hình có endpoint và error state tương ứng.
- Phụ thuộc: P01-02.

### P01-04 — Khởi tạo nền tảng ứng dụng

- Phạm vi file: `was/go.mod`, `was/cmd/api/`, `was/cmd/worker/`, `was/internal/infrastructure/`, `was/internal/middleware/`, `viewer/`, Dockerfile/Compose, `.env.example`, `.gitignore`.
- Thực hiện: chọn/khóa dependency, Gin, React/TypeScript/Tailwind, config validation, `slog`, request ID, error mapping, context/timeout và graceful shutdown.
- Compose mặc định chạy app/viewer/postgres; khai báo Redis/RabbitMQ/worker qua profile hạ tầng trước khi chúng có chức năng. Phase 05 đưa toàn stack vào đường chạy mặc định.
- Tách liveness/readiness; chỉ dependency bắt buộc mới làm mất readiness, dependency tùy chọn lỗi phải hiện trạng thái suy giảm.
- Đầu ra: khung ứng dụng build/run thật, chưa chứa nghiệp vụ giả được đánh dấu hoàn thành.
- Kiểm tra: health thực tế, missing config fail rõ, SIGTERM tắt tiến trình có kiểm soát; backend/frontend build thành công.
- Phụ thuộc: P01-02/03.

### P01-05 — Migration, seed và test harness

- Phạm vi file: `was/migrations/`, `scripts/`, `was/tests/integration/`, `was/Makefile`, `Makefile` điều phối và viewer test config.
- Thực hiện: golang-migrate với migration đầu tiên hợp lệ, seed runner lặp được và migration chạy qua bước riêng trước API, tránh nhiều replica tranh chạy.
- Chuẩn hóa `make migrate-up`, `make seed`, `make test`, `make test-integration`, `make test-e2e`; chọn runner viewer/E2E và cách cấp dịch vụ test thật cô lập.
- Đầu ra: PostgreSQL integration fixture tạo/đọc/xóa dữ liệu thật, tài liệu môi trường test và guard cho script reset.
- Kiểm tra: DB trống migrate được, chạy lại không lỗi, seed không trùng; test runner không chạm DB cần giữ. Không viết test chỉ assert placeholder.
- Phụ thuộc: P01-04.

### P01-06 — Hướng dẫn chạy và theo dõi kết quả

- Thực hiện: tạo `README.md`, `docs/PROGRESS.md`, `docs/known-issues.md`; theo dõi ID `Pxx-nn`, lệnh và vị trí evidence, giới hạn hiện tại.
- Kiểm tra: thực hiện README từ trạng thái clean có kiểm soát; API/frontend hiển thị kết nối thật, không claim module chưa có.
- Phụ thuộc: P01-04/05.

## 4. Dữ liệu, lỗi và rollback

Chỉ áp dụng migration đã có thiết kế; bảng nghiệp vụ triển khai dần theo phase sở hữu. Thiếu secret/config hoặc migration lỗi thì fail rõ, không chạy trên schema sai. Không commit secret, không log token/password. Thử rollback trên DB test; dữ liệu cần giữ phải backup trước thao tác phá hủy. Thay đổi thiết kế phải cập nhật plan bị ảnh hưởng.

## 5. Tiêu chí hoàn thành

- [x] Requirements, architecture, ERD, state machine và OpenAPI nhất quán.
- [x] Quyết định ảnh hưởng Phase 02 đã rõ; điểm mở cho phase sau có nơi theo dõi.
- [ ] API/viewer/PostgreSQL chạy qua Compose; migration/seed/test harness dùng được.
- [x] Build, health/shutdown và kiểm tra nền tảng có evidence thật.
- [x] Không module nghiệp vụ giả được báo hoàn thành.

## 6. Checklist review

- [ ] Mỗi công việc có đầu ra, dependency và cách kiểm chứng.
- [ ] Contract, transaction và quyền được thiết kế trước mã nguồn liên quan.
- [ ] Migration/rollback, dữ liệu test, secret và failure path đã xem xét.
- [ ] Docs/progress phản ánh đúng khả năng đã kiểm chứng.

Tiếp theo: [Phase 02](02-identity-catalog-inventory-plan.md).

## 7. Kết quả triển khai

### Lần 1 (2026-11-25)

- Đã thực hiện P01-01–06: thiết kế, 4 endpoint hiện thực, Go API/worker/migrate/seed/healthcheck, React UI, Docker/Compose, test harness và README.
- Verification: 7 Go unit/race, 2 PostgreSQL integration, 2 frontend unit, 1 Chrome E2E và 9 runtime smoke checks đạt; lint/typecheck/build/vet/OpenAPI/npm ci đạt.
- Review: sub-agent độc lập đã rà và executor đã sửa các findings; xem [review](../docs/review-phase-01.md) và [progress/evidence](../docs/PROGRESS.md).
- Kết quả: mã nguồn và native verification hoàn tất. Tiêu chí chạy Compose chưa đánh dấu vì chưa có quyền daemon; không coi phase đã nghiệm thu đầy đủ.

### Lần 2 (2026-11-25) — Tách WAS và Viewer

- Theo yêu cầu người dùng, toàn bộ backend chuyển vào `was/`, frontend vào `viewer/`; giữ docs/plans/scripts điều phối ở root.
- OpenAPI và file embed chuyển vào `was/api/`; viewer sinh type từ contract này. Backend giữ module path để tránh thay API/import nghiệp vụ ngoài phạm vi.
- Mỗi phần có công cụ build riêng: `was/Makefile`, `viewer/package.json`; root Makefile điều phối và E2E runner nối hai phần.
- Dockerfile và đường dẫn Compose được cập nhật theo cấu trúc mới; tạm thời không chạy kiểm tra Docker theo yêu cầu. Kết quả kiểm chứng ngoài Docker được ghi trong progress.
