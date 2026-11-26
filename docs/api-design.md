# Thiết kế API

Nguồn chuẩn máy đọc: [openapi.yaml](../was/api/openapi.yaml). Đây là JSON hợp lệ trong cú pháp YAML để cả Go embed và tool frontend dùng trực tiếp. Contract toàn Base được chia `x-phase` và `x-implemented`; Phase 01–03 hiện thực foundation/auth/users/catalog/inventory/cart/orders/payments; chỉ các endpoint phase sau chưa triển khai trả404.

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

## Hiện thực Phase 02

Có36 operations đã hiện thực (4 Phase01 +32 Phase02), gồm GET admin/products/{id} cho cả Draft/Inactive. Inventory list thêm product_name/sku đọc từ catalog; adjustment response vẫn có product_id/quantity/version. Tất cả input JSON từ chối unknown field, null cho field bắt buộc và payload ngoài giới hạn; không nhận stock qua product update hoặc role/status qua profile/register.

Password min12 Unicode characters/max72 UTF-8 bytes; email trim/lowercase, SKU trim/uppercase. Các cookie mutation yêu cầu trusted Origin; public đọc catalog không cần phiên. Hạn mức trả429 + Retry-After, Redis lỗi trả503. User role/status/session_version được đọc lại cho protected request.

Page 1–100000, page_size1–100; sort users chỉ created_at/name/email, product chỉ created_at/name/price_asc/price_desc. q tối đa200 ký tự và %/_/backslash được tìm như ký tự literal. Hết hàng suy ra từ inventory; admin CRUD dùng soft-delete/archive, không mất stock history.

## Hiện thực Phase 03

Contract có 63 operations, 50 hiện thực khi bật mock local/test (4 nền tảng, 32 Phase 02, 14 Phase 03). Bổ sung GET admin/orders/{id}; các API cart/order/cancel/status/attempt/simulate đã chạy thật. Khi tắt mock, 2 route attempt/simulate không đăng ký và trả 404.

Cart trả tên/SKU/image/version/giá/stock/availability cùng quote_hash, pricing_valid và mock_payments_enabled. POST item tăng quantity, PUT thay quantity tuyệt đối; DELETE item/clear trả 204. Chặn hơn 100 sản phẩm hoặc quantity ngoài 1–1000. Coupon không có trong CheckoutInput Phase 03; unknown field bị từ chối.

Checkout gửi address_id/cart_version/quote_hash/payment_method, không gửi total. Server khóa cart/product/stock rồi tính lại. CART_CHANGED/QUOTE_CHANGED/PRODUCT_UNAVAILABLE trả409 và yêu cầu xem lại; cart/stock/order/payment/outbox/idempotency không thay đổi khi thất bại. Key hợp lệ 16–128 chữ/số/gạch nối/gạch dưới, namespace actor/operation và fingerprint chứa resource ID; status replay là201 cho checkout/attempt, 200 cho lifecycle/simulate.

Order trả snapshot địa chỉ ba trường recipient/phone/address, item name/SKU/image/giá, tổng, history và payment kèm attempts. Customer chỉ xem/hủy đơn của mình; Admin có list/detail/status riêng. Operation admin transition tách khỏi customer cancel để không replay dữ liệu ngoài quyền sau khi role đổi.

Mock attempt chỉ tạo khi order/payment Pending và chưa có attempt Pending/Paid; tối đa5lần. Simulate Success ghi Paid/Confirmed/payment.completed, Failure giữ aggregate/order Pending. Callback trễ hoặc attempt final với intent mới trả409; cùng key đã commit trả snapshot cũ. Không có callback payment gateway công khai trong Base.
