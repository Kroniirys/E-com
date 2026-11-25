# Thiết kế API

Nguồn chuẩn máy đọc: [openapi.yaml](../was/api/openapi.yaml). Đây là JSON hợp lệ trong cú pháp YAML để cả Go embed và tool frontend dùng trực tiếp. Contract toàn Base được chia `x-phase` và `x-implemented`; Phase 01 chỉ hiện thực foundation/health/spec, endpoint nghiệp vụ chưa triển khai trả 404.

## Quy ước

- API nghiệp vụ `/api/v1`; health và OpenAPI ngoài prefix. ID UUID, ngày RFC3339 UTC, tiền int64 nguyên đồng VND. Frontend phải kiểm số tiền trong miền số nguyên an toàn JS hoặc biểu diễn chuỗi nếu mở rộng giới hạn; Base giới hạn tổng <= 9.000.000.000.000 đồng.
- Thành công: `{data: ...}`; lỗi: `{error: {code, message, request_id}}`. 204 không body. Request ID có trong header `X-Request-ID`.
- 400 input sai, 401 chưa xác thực, 403 thiếu quyền, 404 tài nguyên không tồn tại/không được thấy, 409 conflict/version/stock/idempotency, 429 rate limit, 503 dependency/schema chưa sẵn sàng.
- List: page >= 1, page_size 1–100, mặc định 20; response có items/page/page_size/total. Sort theo allowlist và tie-break ID ổn định. Search giới hạn 200 ký tự.
- Mutation checkout/cancel/mock-result/adjustment bắt buộc `Idempotency-Key` độ dài 16–128; namespace actor+operation. Cùng key/payload trả kết quả cũ; khác payload 409. Không lưu key duy nhất trong Redis.
- Checkout request chứa cart_version và quote_hash từ GET cart; server kiểm lại, không tin client gửi total/discount/stock.

## Auth và ownership cho Phase 02

Access Bearer token trong memory, refresh cookie HttpOnly/Path=/api/v1/auth/SameSite=Lax/Secure ngoài local. Login/refresh cấp và rotate bằng Set-Cookie, logout/change-password thu hồi và clear cookie (Max-Age=0). Mutations dùng cookie phải kiểm Origin allowlist; không để GET mutate. Cùng-origin qua proxy là mặc định. Response không lộ hash/token refresh.

Public đọc catalog/review visible. Customer chỉ đọc/sửa profile/address/cart/order/payment/notification của mình. Admin CRUD catalog/users/inventory/coupon/moderation/dashboard và chuyển trạng thái order. Không nhận role/status/user_id qua mass assignment. Token hết hạn hoặc session/user bị khóa phải từ chối dù UI vẫn có state cũ.

## Mapping UI → API theo phase

| UI | API | Phase |
| --- | --- | --- |
| Trạng thái nền tảng | health, foundation, OpenAPI | 01 |
| Login/profile/address | auth, me, me/addresses | 02 |
| Storefront và admin catalog | products/categories, admin/products/admin/categories | 02 |
| User/inventory admin | admin/users, admin/inventory | 02 |
| Cart/checkout/order | cart/items, orders, orders/{id}/cancel | 03 |
| Payment demo | orders/{id}/payment-attempts, payment-attempts/{id}/simulate | 03 |
| Admin fulfillment | admin/orders, admin/orders/{id}/status | 03 |
| Coupon/review/dashboard | coupons/validate, admin/coupons, product reviews, admin/reviews, admin/dashboard | 04 |
| Notification | notifications, notifications/{id}/read | 05 |

## Contract kiểm chứng

OpenAPI validation và generation TypeScript chạy qua `make contract`. Phase 01 client foundation lấy type từ generated schema, không tự định nghĩa response khác. Backend chỉ đăng ký endpoint đã triển khai. Swagger UI local hiển thị đánh dấu endpoint planned trong mô tả; tài liệu tồn tại không đồng nghĩa tính năng đã có.
