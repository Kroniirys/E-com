# Checklist thực thi Phase 04

Đầu vào: plan04 và mã Phase03 đã push/merge develop tại12ec68d. E-com dùng plan fullstack tại plans và thiết kế docs; không áp gate projects.toml/approved task của TeemStone. Không tự phê duyệt artifact. Docker tiếp tục tạm hoãn.

| Bước | File chính | Công việc / kiểm chứng |
| --- | --- | --- |
| 1 | was/internal/modules/coupons/policy_test.go, policy.go | Test trước cho percent/fixed/minimum/expiry/limit/rounding và miền tiền |
| 2 | was/migrations/000004_coupons_reviews_admin.*, internal/database/database.go | Schema4, coupon usage/review unique, nâng cấp order cũ và rollback có guard |
| 3 | modules/coupons/coupons.go, orders/orders.go, httpapi/engagement.go | Coupon CRUD/archive/preview; checkout khóa coupon trước inventory, snapshot/hash/usage/result cùng commit |
| 4 | modules/reviews/reviews.go | Delivered eligibility, unique cả deleted, public visibility/aggregate, owner delete và admin moderation |
| 5 | modules/admin/dashboard.go, httpapi/engagement.go | Dashboard RepeatableRead, [from,to) theo IANA timezone, exact decimal aggregate |
| 6 | was/api/openapi.yaml, cmd/api/main.go, tests/integration/phase04_test.go | Contract/wiring, coupon race/fault injection/review/RBAC/dashboard/migration |
| 7 | viewer/src/pages/{CartPage,OrdersPage,ProductPage,AdminPage}.tsx, EngagementAdmin.tsx, ProductReviews.tsx, intent.ts | UI coupon/preview, review/moderation/dashboard và lỗi/retry |
| 8 | was/internal/bootstrap/phase04.go, cmd/seed/main.go, viewer/e2e/phase04.spec.ts | Seed opt-in có Delivered thật, không đụng cart đang có; browser flow đầy đủ |
| 9 | docs và plans/04-* | Review độc lập, sửa lặp, evidence và cập nhật kết quả |

Code coupon upper/trim, chỉ chữ/số/gạch nối/gạch dưới, 1–64 ký tự. Percentage1–100, fixed1..9×10¹² VND, minimum0..9×10¹²; usage limit toàn cục. Preview hash chứa base cart quote + coupon ID/version/discount/total; tăng used_count không thay version điều khoản. CheckoutInput coupon_code omitempty để fingerprint request Phase03 không đổi. Hủy không hoàn lượt; không coupon stacking hoặc giảm shipping.

Khóa checkout: idempotency → user NO KEY UPDATE → cart → products theo ID → coupon → inventory theo ID. Moderation khóa role Admin trước review; customer review create khóa user rồi product, không khóa order sau product vì Delivered là terminal. Không phát thêm event coupon/review khi chưa có consumer.

Dashboard count user/product chưa soft-delete theo created_at, count order mọi trạng thái theo created_at; tổng tiền chỉ order Delivered và payment Paid trong cùng khoảng ngày tạo đơn. from/to là ngày theo timezone IANA, to loại trừ; mặc định không lọc và timezone UTC. Tổng tiền trả chuỗi thập phân chính xác vì tổng nhiều order có thể vượt trần một order và Number JavaScript.

## Implementation Result — Lần 1 (2026-11-26)

Hoàn thành bước1–9, có review độc lập và evidence tại [progress](PROGRESS.md). Phase03 đã push develop trước khi triển khai Phase04. Coupon rollback/concurrency, legacy replay, review/moderation, dashboard và opt-in seed có kiểm chứng thật. UI Account có danh sách review riêng để xóa cả khi sản phẩm không public. Không tự thêm event coupon/review. Docker tạm hoãn.
