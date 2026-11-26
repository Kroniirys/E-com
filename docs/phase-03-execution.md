# Checklist thực thi Phase 03

Phạm vi: thực hiện P03-01–06 theo yêu cầu chủ dự án trên E-com. Docker tiếp tục tạm hoãn. Plan fullstack và thiết kế tại `docs/` là đầu vào; E-com không dùng projects.toml hoặc gate approved/task của workspace TeemStone. Không tự đổi trạng thái phê duyệt.

| Bước | File chính | Công việc / kiểm chứng |
| --- | --- | --- |
| 1 | was/migrations/000003_cart_orders_payments.*, internal/database/database.go | Schema v3, FK/check/unique, nâng cấp và rollback trên schema test |
| 2 | internal/platform/money/*, modules/orders/policy* | Unit test trước cho tính tiền giới hạn và transition; xác nhận lỗi trước khi hiện thực |
| 3 | internal/platform/idempotency/*, internal/platform/outbox/* | Transaction/retry tối đa 3 lần với 40001/40P01, result cùng commit; outbox bền chưa publish |
| 4 | internal/modules/cart/cart.go | Cart/version/quote, mutation khóa cart, tối đa 100 item và quantity 1000; kiểm chứng ownership/giá/stock |
| 5 | internal/modules/orders/orders.go, modules/payments/payments.go | Checkout/snapshot/stock/history, lifecycle/attempt nguyên tử; khóa và concurrency qua PostgreSQL |
| 6 | was/api/openapi.yaml, internal/httpapi/commerce.go, cmd/api/main.go, config/security.go | Contract trước UI, đăng ký route, mô phỏng chỉ khi bật local/test; chặn ở production |
| 7 | viewer/src/pages/{CartPage,OrdersPage}.tsx, ProductPage.tsx, App.tsx, AdminPage.tsx | UI COD/mock, retry giữ key, history/detail/cancel/admin transition, loading/error và xác nhận lại quote |
| 8 | was/tests/integration/phase03_test.go, viewer/e2e/phase03.spec.ts | 20 customer tranh stock 5, rollback/fault injection, replay/restart/payment race, browser xuyên suốt |
| 9 | docs/{erd,api-design,order-payment-state-machine,events,PROGRESS,review-phase-03}.md, plans/03-* | Review độc lập, sửa lặp, evidence và tiêu chí hoàn tất |

Quote SHA-256 trên cart/version, product ID/version/giá/quantity và shipping lấy từ DB. Checkout so khớp lại, không tin tiền client. Cart hiển thị live availability để khách xóa món đã archive/hết stock. Key giữ trong vòng đời dữ liệu Base, chưa tự dọn; payload gồm resource ID để chống replay sang intent khác. Không nhận coupon_code ở Phase 03.

Khóa: idempotency → user NO KEY UPDATE (checkout snapshot địa chỉ) → cart → product theo ID → inventory theo ID. Cart mutation khóa cart rồi product; không khóa user sau cart. Lifecycle/payment: idempotency → order → payment/attempt → inventory theo ID; không khóa cart/user sau order. Không gọi mạng trong transaction. Các snapshot đã commit bất biến trước sửa product/address/settings.

## Implementation Result

### Lần 1 (2026-11-26)

Đã hoàn thành bước1–9 trên PostgreSQL/Redis/Chrome native. Review độc lập và các sửa lỗi có regression được ghi ở [review](review-phase-03.md). [Progress](PROGRESS.md) chứa số test/lệnh/evidence, gồm kill/restart process và UI lost-response recovery. E2E runner tách từng spec thành schema/API/Redis prefix riêng để giữ nguyên policy auth.

Mã ở branch `feat/phase-03-cart-orders-payments`. Chưa có yêu cầu commit/push riêng cho Phase03; không thay lịch sử develop hoặc tự publish. Docker tiếp tục tạm hoãn, không có ước lượng thời gian.
