# Yêu cầu E-commerce Base

Nguồn: [plan tổng](../plans/00-master-plan.md). Lệnh triển khai Phase 01 và Phase 02 của chủ dự án là phạm vi đã thực thi hiện tại. Các lựa chọn đơn giản hóa dưới đây là quyết định triển khai theo phương án đã đề xuất trong plan, không phải yêu cầu mới được suy ra từ mã nguồn.

## Phạm vi và truy vết

| ID | Yêu cầu | Phase | Cách nghiệm thu |
| --- | --- | --- | --- |
| FND-01 | Go/Gin/GORM/PostgreSQL, migration, config, logging, health, shutdown | 01 | Unit, integration DB, smoke HTTP, SIGTERM |
| FND-02 | React/TypeScript/Tailwind, trạng thái kết nối API thật | 01 | Build, component test và browser E2E |
| FND-03 | Docker Compose, seed lặp, OpenAPI, hướng dẫn chạy | 01 | Clean-start, validate contract, seed/integration |
| AUTH-01 | Register/login/refresh/logout/change-password/me, Customer/Admin | 02 | API/UI và token/permission negative test |
| USER-01 | Profile/address riêng, admin CRUD/search/status/pagination | 02 | Ownership, khóa tài khoản và admin UI |
| CAT-01 | Product/category CRUD, SKU, ảnh URL, giá/status/search/filter/sort | 02 | Constraint, visibility, UI và dữ liệu thật |
| INV-01 | Adjustment/history, stock không âm và chống overselling | 02/03 | Concurrent transaction và đối chiếu SQL |
| CART-01 | Cart theo user, lưu PostgreSQL, không giữ hàng | 03 | CRUD/ownership, cart edit đua checkout |
| ORD-01 | Checkout nguyên tử, snapshot, lifecycle/history/cancel | 03 | Transaction rollback, idempotency, E2E |
| PAY-01 | COD/mock, attempts/retry, chỉ một thanh toán thành công | 03 | Concurrent payment/cancel, callback replay |
| CPN-01 | Coupon fixed/percentage, expiry/minimum/limit/enable-disable | 04 | Race lượt cuối, preview cũ, giá trị biên |
| REV-01 | Review Delivered, rating 1–5, owner/admin moderation | 04 | Eligibility/unique/visibility và E2E |
| ADM-01 | Dashboard dữ liệu thật, các CRUD theo module | 02–04 | SQL fixture và quyền server |
| ASY-01 | Outbox, RabbitMQ/worker, retry/DLQ/dedupe/recovery | 03–05 | Fault injection và event trace |
| NTF-01 | Notification nội bộ/mock email và UI đúng user | 05 | Duplicate/restart/ownership |
| OPS-01 | Redis cache/rate limit, Prometheus, k6, handover | 02/05/06 | Fallback, cache race, report tái lập |

## Quy tắc nghiệp vụ đã chọn cho Base

- Một SKU/product, một kho, một người bán, tiền VND nguyên đồng bằng int64/bigint; không float. Mọi phép nhân/cộng/giảm kiểm tra tràn số và kết quả không âm.
- Shipping cố định lấy từ cấu hình cửa hàng và snapshot vào order. Chưa có thuế, hãng vận chuyển, biến thể hoặc upload file. Ảnh là metadata/URL, không backend-fetch URL tùy ý.
- Draft/Active/Inactive được lưu; hết hàng suy từ inventory. Public chỉ thấy Active, gồm Active hết hàng.
- Cart không giữ hàng. Checkout kiểm tra version cart và quote giá, đọc lại giá/trạng thái/stock từ DB; lệch thì 409 và cần xác nhận lại.
- Trừ stock khi tạo order; hủy hợp lệ/Failed cuối cùng hoàn đúng một lần. Không reservation hết hạn.
- COD Paid khi Delivered. Mock Paid chỉ khi attempt thành công; attempt thất bại cho phép retry khi order Pending. Không hoàn tiền online, không hủy order đã Paid/Shipped.
- Một coupon/order; discount chỉ trên subtotal, làm tròn xuống đồng, không vượt subtotal; minimum xét subtotal trước giảm, expiry UTC, usage limit toàn cục. Không hoàn lượt khi hủy.
- Review cần order Delivered chứa product; một review/user/product kể cả bản ghi soft-delete. Moderation ẩn/hiện; rating public chỉ tính visible.
- PostgreSQL là nguồn chuẩn. Cache/broker lỗi không làm sai tiền, stock hoặc quyền. Outbox lưu cùng transaction; delivery at-least-once và consumer chống trùng.

## Bảo mật và điều kiện chung

Không commit/log secret, password, token hoặc địa chỉ đầy đủ. Quyền/ownership kiểm tra tại server. Chặn mass assignment và SQL sort tùy ý. Seed demo không tự ghi đè dữ liệu người dùng. Script reset chỉ dành DB test có guard.

Đến Phase 02, AUTH-01, USER-01, CAT-01 và phần adjustment/history của INV-01 đã triển khai và kiểm chứng. ADM-01 có CRUD user/catalog/inventory; dashboard tổng hợp và giao dịch checkout chưa thuộc phase này. OPS-01 đã có Redis rate limit và hành vi lỗi dịch vụ; cache/metrics/load test còn ở phase sau. Xem [bằng chứng](PROGRESS.md). Endpoint Phase 03 trở đi được đánh dấu tương lai trong OpenAPI và chưa đăng ký. Không dùng mock dữ liệu để thay chức năng thật.
