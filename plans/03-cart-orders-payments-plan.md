---
title: "Phase 03 — Giỏ hàng, đơn hàng và thanh toán"
artifact: implementation-plan
role: fullstack
status: draft
version: 0.2
owner: "TBD"
approvers: []
last_updated: 2026-11-25
---

# Phase 03 — Giỏ hàng, đơn hàng và thanh toán

## 1. Mục tiêu và đầu vào

Bàn giao luồng mua hàng cốt lõi từ UI tới PostgreSQL, có COD/mock payment và kiểm chứng concurrency. Đầu vào: [plan tổng](00-master-plan.md), thiết kế state machine và [Phase 02](02-identity-catalog-inventory-plan.md) đạt tiêu chí.

## 2. Phạm vi và quy tắc giao dịch

Cart lưu PostgreSQL; checkout tính giá phía server, snapshot item/address/shipping, tạo order, giảm stock, ghi history, payment và outbox trong cùng transaction. Chưa dùng coupon/review và chưa publish RabbitMQ; outbox được lưu bền để Phase 05 xử lý.

State machine dưới đây là phương án triển khai theo quyết định Phase 01; nếu thay phải cập nhật contract/test trước:

| Thao tác | Điều kiện và kết quả |
| --- | --- |
| Tạo order | Pending; payment Pending; giảm stock ngay |
| Xác nhận | Pending → Confirmed; admin xác nhận COD hoặc mock payment thành công xác nhận trong cùng transaction |
| Fulfillment | Confirmed → Processing → Shipped → Delivered, admin; COD đánh Paid cùng transaction Delivered; mock phải Paid trước Processing |
| Hủy | Pending/Confirmed và payment chưa Paid → Cancelled; hủy payment còn Pending, hoàn stock một lần |
| Failed cuối cùng | Admin chuyển Pending → Failed khi chưa Paid; hủy payment còn Pending, hoàn stock một lần |
| Mock thất bại | Attempt Failed, payment aggregate vẫn Pending, order vẫn Pending; được tạo attempt mới theo chính sách retry |
| Mock thành công | Một attempt Paid, payment Paid; cấm tạo success thứ hai; nếu order đã hủy/Failed thì từ chối |

Một attempt chỉ chuyển Pending → Paid/Failed/Cancelled. Không chuyển attempt Failed trở lại Pending. Không hỗ trợ hoàn tiền online hoặc hủy sau Shipped trong Base.

## 3. Công việc

### P03-01 — Schema giao dịch

- Phạm vi file: `was/migrations/`, repositories cart/orders/payments, shared idempotency và outbox.
- Tạo carts/cart_items/orders/order_items/payments/payment_attempts/outbox_events; tái sử dụng inventory history/idempotency từ Phase 02.
- Unique cart/user và cart-item/product, payment aggregate/order; constraint hoặc index thích hợp chặn nhiều successful attempt/order. Snapshot không bị mất khi product/user/address thay đổi.
- Thiết kế idempotency key lưu actor/operation/payload fingerprint/result; record commit cùng giao dịch. Chốt giới hạn/retention và semantics key trước triển khai, không tự xóa khi client còn được phép replay.
- Kiểm tra: migration trên DB Phase 02, FK/unique/check và rollback test cô lập.
- Phụ thuộc: Phase 02 và state machine đã chốt.

### P03-02 — Cart và tính tiền

- Phạm vi file: `was/internal/modules/cart/`, money/pricing helpers.
- GET cart, thêm/sửa/xóa item, clear cart; quantity nguyên dương có giới hạn, product Active và stock đủ tại thời điểm thao tác. Cart không giữ stock.
- Subtotal/discount/shipping/total dùng số nguyên có kiểm tra tràn số. Phase này discount bằng 0 theo phạm vi, không giả lập coupon.
- Checkout đọc lại giá/stock; nếu giá thay đổi so với bản khách đã thấy thì trả conflict để xác nhận lại, không âm thầm tính khác. Chốt request price/version token trong OpenAPI.
- Kiểm tra: ownership, product đổi trạng thái/giá, quantity sai, cart rỗng, item bị archive và tổng tiền biên.
- Phụ thuộc: P03-01.

### P03-03 — Checkout nguyên tử và chống trùng

- Phạm vi file: `was/internal/modules/orders/`, inventory service và outbox repository.
- Xác thực user/address/ownership; claim idempotency key; khóa cart theo user để serialize checkout và cart mutation. Khóa stock theo thứ tự product ID ổn định.
- Tái kiểm tra cart version/giá/trạng thái/stock; tính tổng, tạo snapshot/order/payment, trừ stock và ghi inventory_transactions/outbox; clear cart, lưu idempotent response rồi commit.
- Một bước lỗi phải rollback mọi thay đổi. Dùng guarded update/check constraint chống âm; retry lỗi deadlock/serialization theo chính sách giới hạn, giữ nguyên key.
- Cùng user/cùng key đồng thời chỉ có một kết quả; cùng key khác payload trả 409; key khác trên cùng cart snapshot không tạo thêm đơn sau khi cart đã checkout.
- Kiểm tra: stock thấp bị nhiều checkout tranh, multi-item thiếu một món rollback toàn bộ, cart edit đua checkout, mất response rồi retry, product/admin adjustment đua checkout.
- Phụ thuộc: P03-01/02.

### P03-04 — Lifecycle, cancel và payment

- Phạm vi file: `was/internal/modules/orders/`, `was/internal/modules/payments/` và admin routes.
- Triển khai history/detail theo ownership, admin list/status, transition đúng bảng và hủy hợp lệ có idempotency.
- COD không đánh Paid khi mới đặt hàng. Mock có endpoint tạo attempt và mô phỏng kết quả trong local/demo, ownership/auth rõ; cấu hình tắt mô phỏng phải chặn route.
- Payment/cancel/fulfillment khóa order trước, sau đó payment/stock theo thứ tự thiết kế. Success/cancel/Failed cùng cập nhật order/payment/stock/history/outbox nguyên tử.
- Attempt retry chỉ khi order còn Pending và chưa Paid; không tạo song song nhiều attempt Pending, không cho callback trễ thanh toán lại đơn đã hủy.
- Kiểm tra: replay callback, success/failure trái thứ tự, cancel đua payment, Delivered COD gửi lại, user xem đơn người khác và transition sai.
- Phụ thuộc: P03-03.

### P03-05 — Customer/admin UI mua hàng

- Phạm vi file: `viewer/src/` cart/checkout/orders/payment/admin orders.
- Customer: cart, địa chỉ/shipping, xác nhận giá, chọn COD/mock, kết quả success/failure/retry, lịch sử/chi tiết/hủy đơn.
- Admin: list/detail order và action chuyển trạng thái hợp lệ. Tổng hiển thị lấy từ server; nút disable chống double-click chỉ hỗ trợ UX, không thay idempotency server.
- Giữ key khi retry cùng thao tác; thay key cho intent mới. Khi checkout xung đột giá/stock, tải lại và yêu cầu người dùng xác nhận lại.
- Kiểm tra: đi hết hai phương thức payment qua UI; lỗi mạng và double submit không tạo trùng.
- Phụ thuộc: P03-02–04.

### P03-06 — Chứng minh tính đúng và tài liệu

- Unit: tiền, state transition, eligibility cancel/retry. Integration: transaction rollback, idempotency, payment/cancel, FK và snapshot.
- Concurrency test dùng nhiều connection PostgreSQL thật qua đường API/service thật; `go test -race ./...` là kiểm tra race trong Go, không thay thế test database.
- Fixture stock 5 với 20 customer checkout độc lập mỗi người 1 món: đúng 5 đơn thành công, stock 0, không history/outbox thừa; thêm test một user gửi nhiều request trên cùng cart.
- Test hủy hai lần chỉ hoàn một lần; payment hai success chỉ một Paid; broker chưa chạy vẫn tạo order/outbox bền. Test process crash/mất response rồi retry trả lại order cũ.
- Đầu ra: evidence SQL đối chiếu order/payment/inventory/outbox, cập nhật OpenAPI/ERD/state machine/progress và E2E customer/admin.
- Phụ thuộc: P03-01–05.

## 4. Lỗi, observability và rollback

Log request/order/attempt/event ID, không log token/address đầy đủ. Ghi lỗi stock conflict/duplicate/transition invalid để kiểm tra; Phase 05 bổ sung metrics. Outbox tồn chưa gửi là khả năng đang chờ Phase 05, không báo đã có notification. Rollback app phải tương thích schema; không xóa order/payment/history/outbox để chạy lại. Dữ liệu giao dịch lỗi cần phục hồi có đối chiếu và migration sửa tiến.

## 5. Tiêu chí hoàn thành

- [ ] Customer tạo và xem order từ UI bằng COD hoặc mock; admin xử lý đúng lifecycle.
- [ ] Giá/address/shipping là snapshot; server quyết định tổng tiền.
- [ ] Không overselling, stock âm, order/payment/hoàn kho trùng trong test concurrency.
- [ ] Retry/payment failure/cancel và callback trễ có kết quả rõ, đúng state machine.
- [ ] Mutation và outbox/idempotency commit cùng transaction; dữ liệu tồn tại sau restart.
- [ ] Unit/integration/concurrency/E2E và docs có evidence thật.

## 6. Checklist review

- [ ] Khóa, transaction boundary, retry và idempotency được kiểm tra cả failure path.
- [ ] Payment aggregate/attempt/order không bị gộp trạng thái sai.
- [ ] Mọi mutation quyền và ownership kiểm tra phía server.
- [ ] Không đánh dấu coupon/notification đã hoàn thiện trước phase sở hữu.

Tiếp theo: [Phase 04](04-coupons-reviews-admin-plan.md).
