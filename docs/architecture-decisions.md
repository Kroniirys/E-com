# Quyết định kiến trúc

Các quyết định này là lựa chọn triển khai cho Base dựa trên phương án của plan. Có thể điều chỉnh trước phase liên quan, đồng thời cập nhật contract và test; không coi đây là phê duyệt artifact tự động.

| ID | Quyết định | Lý do / hệ quả |
| --- | --- | --- |
| ADR-001 | Modular monolith + process worker riêng | Một transaction DB cho checkout, ranh giới module qua service; chưa microservices |
| ADR-002 | PostgreSQL + GORM; golang-migrate chạy riêng | Không AutoMigrate, không nhiều API tự tranh migrate; readiness kiểm schema đúng version |
| ADR-003 | Phase 01 chỉ migrate store_settings | Dữ liệu cấu hình cửa hàng có giá trị thật; schema auth/catalog triển khai Phase 02 theo ERD tổng thể |
| ADR-004 | VND int64/bigint, shipping cấu hình, snapshot order | Server quyết định tổng; kiểm tràn số; không float/thuế/phí động |
| ADR-005 | Stock trừ ngay, khóa có thứ tự, hoàn đúng một lần | Đơn giản hơn reservation; mọi mutation stock/history/order/outbox cùng transaction |
| ADR-006 | Cart PostgreSQL, quote/cart version | Không mất cart khi Redis lỗi; giá thay đổi cần khách xác nhận lại |
| ADR-007 | Idempotency theo actor/operation/key và payload hash | Result record cùng transaction, không tách cache key với DB commit; giữ key trong vòng đời dữ liệu nghiệp vụ Base |
| ADR-008 | Payment aggregate tách attempt | Failure của attempt không làm order Failed tự động; một successful attempt/order |
| ADR-009 | Outbox + publisher confirms + consumer inbox/dedupe | Có thể phát lại, không mất sự kiện đã commit; không hứa exactly-once cho email |
| ADR-010 | URL ảnh; không upload/fetch từ backend | Chưa tạo bề mặt upload/SSRF; nếu đổi cần plan và security test riêng |
| ADR-011 | Coupon không stacking/hoàn lượt; review một/user/product | Giảm phạm vi nhưng vẫn có test concurrency và eligibility |
| ADR-012 | Refresh cookie HttpOnly, access token trong memory | Cùng origin qua proxy; SameSite=Lax, Secure ngoài local, Origin check cho cookie mutation; thêm CSRF token nếu hỗ trợ cross-site |
| ADR-013 | bcrypt cost 12 cho Phase 02, JWT HS256 allowlist | Secret tối thiểu 32 byte khi auth được triển khai; access TTL 15 phút, refresh 7 ngày; session revoked/disabled user được kiểm DB mỗi request nhạy cảm |
| ADR-014 | Auth rate limit Redis fail-closed; catalog fail-open về DB | Phase 02 login 5 lần/phút/IP; register 5 lần/phút/IP; refresh 20 lần/phút/session; cấu hình rõ, không tin X-Forwarded-For nếu chưa allow proxy |
| ADR-015 | Cache có version/generation + TTL cấu hình | Detail 60 giây, list 30 giây là chính sách kỹ thuật đề xuất Phase 05; public visibility luôn kiểm nguồn chuẩn |
| ADR-016 | Vitest/Testing Library và Playwright | Component test cho UI state, browser E2E gọi backend/database thật; không dùng API mock để báo tích hợp pass |
| ADR-017 | Integration schema riêng trong database tên *_test | Cleanup chỉ schema ecom_it_ tạo bởi test; thiếu TEST_DATABASE_URL làm test integration fail rõ, không skip |
| ADR-018 | Worker Phase 01 chỉ bootstrap DB/schema và chờ shutdown | Có entrypoint chạy được nhưng chưa consumer/job; không claim sự kiện được xử lý |

## Thứ tự khóa thống nhất cho Phase 03/04

- Checkout: claim idempotency → cart row → product rows theo ID → coupon row nếu có → inventory rows theo product ID → tạo order/payment/history/outbox/result. Product update phải dùng cùng row lock để serialize thay đổi giá/status.
- Cart mutation: khóa cart row trước item; không lấy stock lock để giữ hàng.
- Payment/cancel/fulfillment: claim idempotency → order row → payment/attempt → inventory theo ID khi cần hoàn. Không khóa cart sau order.
- Adjustment: claim idempotency → inventory theo ID. Không lấy product lock sau inventory; không update độc lập giá/status từ adjustment.
- Không gọi mạng/broker khi giữ transaction. Retry deadlock/serialization có giới hạn; uniqueness/idempotency vẫn là lớp bảo vệ cuối.

## Điểm mở

Phase 01 không có blocker thiết kế cho auth/catalog. Trước Phase 05 phải chốt TTL/retry/retention theo thử nghiệm tải, event versioning và concurrency cụ thể. Các giá trị kỹ thuật nêu ở đây là chính sách khởi điểm, không là cam kết hiệu năng. Chủ dự án có thể thay quyết định phạm vi trước module liên quan.
