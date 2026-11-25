---
title: "Phase 04 — Coupon, đánh giá và quản trị"
artifact: implementation-plan
role: fullstack
status: draft
version: 0.2
owner: "TBD"
approvers: []
last_updated: 2026-11-25
---

# Phase 04 — Coupon, đánh giá và quản trị

## 1. Mục tiêu và đầu vào

Hoàn thiện nghiệp vụ mở rộng và màn hình quản trị trên nền luồng mua hàng đã được kiểm chứng. Đầu vào: [plan tổng](00-master-plan.md), [Phase 03](03-cart-orders-payments-plan.md) đạt tiêu chí và quyết định coupon/review ở Phase 01.

## 2. Phạm vi

Coupon percentage/fixed, thời hạn/minimum/usage limit/enable-disable; review mua hàng hợp lệ và moderation; dashboard admin dữ liệu thật. User/product/category/inventory/order admin ở phase trước chỉ bổ sung phần tích hợp, không viết lại.

## 3. Công việc

### P04-01 — Schema coupon/review và contract

- Phạm vi file: `was/migrations/`, modules coupons/reviews/admin và `was/api/openapi.yaml`.
- Tạo coupons/coupon_usages/reviews, FK tới user/product/order; unique review/user/product và coupon usage/order. Normalize coupon code, ràng buộc rating 1–5, tiền/percentage/range hợp lệ.
- Order lưu snapshot code, kiểu/giá trị discount và discount thực tế; thay/xóa coupon không đổi tổng của đơn cũ.
- Contract có coupon validate/list/detail/create/update/enable-disable, review list/create/delete của chủ sở hữu và admin hide/show; chốt pagination và visibility.
- Kiểm tra: DB Phase 03 migrate được, constraint chặn dữ liệu sai và đọc order cũ chưa có coupon vẫn đúng.
- Phụ thuộc: Phase 03.

### P04-02 — Coupon service và tích hợp checkout

- Phạm vi file: `was/internal/modules/coupons/`, pricing và checkout transaction.
- Thực hiện: percentage hoặc fixed amount trên subtotal, tối đa một coupon/order; tổng không âm, discount không vượt subtotal; shipping không được giảm trong phương án Base.
- Percentage dùng phép tính số nguyên và quy tắc làm tròn xuống đơn vị đồng được ghi trong contract/test. Minimum xét subtotal trước giảm, expiry theo UTC; coupon bị disable/hết hạn thì không dùng được.
- Validate chỉ là preview, không giữ lượt. Checkout khóa coupon row và kiểm tra điều kiện/lượt dùng lại trong cùng transaction với order/stock/usage/outbox; thống nhất thứ tự khóa để tránh deadlock.
- Chốt usage limit toàn cục, không tự thêm giới hạn theo user nếu chưa sửa yêu cầu. Theo phương án Base, hủy đơn không hoàn lượt dùng; retry cùng key không trừ lượt thêm.
- Kiểm tra: nhiều checkout tranh lượt cuối, expiry/disable giữa preview và submit, stock fail không tiêu lượt, key replay không dùng coupon hai lần, discount biên và order cũ không đổi giá.
- Phụ thuộc: P04-01 và checkout Phase 03.

### P04-03 — Review và moderation

- Phạm vi file: `was/internal/modules/reviews/`, product detail integration.
- Thực hiện: user có order Delivered chứa product mới được review; một review/customer/product; rating 1–5 và content có giới hạn, xử lý như plain text để tránh script injection.
- Customer xóa review của mình; admin ẩn/hiện kèm lý do. Bản ghi bị xóa mềm vẫn giữ uniqueness theo phương án Base, không tạo nhiều review qua delete/recreate.
- Public chỉ thấy review visible; average rating/count tính trên cùng tập visible; cập nhật sau moderation, không tính review ẩn.
- Kiểm tra: chưa mua, order Cancelled/Shipped chưa Delivered, user khác, post đồng thời, rating/content sai, hide/show và aggregate đúng.
- Phụ thuộc: P04-01 và order lifecycle Phase 03.

### P04-04 — Dashboard và tích hợp quản trị

- Phạm vi file: `was/internal/modules/admin/`, admin API và frontend routes.
- Thực hiện: dashboard dùng truy vấn thật cho tổng user/product/order và tổng tiền của đơn Delivered có payment Paid; không đặt tên chỉ số mơ hồ hoặc tính cả đơn hủy.
- Bộ lọc khoảng dữ liệu/timezone, công thức chỉ số và index được ghi rõ. Không tự mở rộng sang BI hoặc báo cáo tài chính phức tạp.
- Tích hợp admin coupon CRUD/list/enable-disable và review moderation với navigation/scope quyền hiện có; inventory/user/order vẫn thuộc service gốc.
- Kiểm tra: đối chiếu dashboard với SQL fixture gồm Pending/Cancelled/Delivered/Paid, customer bị chặn và không tạo endpoint trùng chức năng.
- Phụ thuộc: P04-02/03.

### P04-05 — UI coupon/review và hồi quy checkout

- Phạm vi file: `viewer/src/` checkout/product/orders/admin dashboard/coupons/reviews.
- Customer nhập/bỏ coupon, xem breakdown preview, nhận lỗi coupon/giá thay đổi và xác nhận lại; hiển thị discount snapshot từ order response.
- Product hiển thị review/rating thật; user đủ điều kiện gửi/xóa review; không dựa riêng vào trạng thái nút để kiểm tra eligibility.
- Admin thao tác coupon/moderation/dashboard; có validation/loading/empty/error/pagination/responsive/keyboard.
- Kiểm tra: E2E admin tạo coupon → customer checkout → admin Delivered → customer review → admin ẩn review → public không còn thấy.
- Phụ thuộc: P04-02–04.

### P04-06 — Kiểm chứng, sự kiện và tài liệu

- Unit: coupon eligibility/discount/rounding, review eligibility/aggregate. Integration: coupon transaction, concurrent usage limit, review unique/ownership và dashboard query.
- Chạy lại bộ concurrency Phase 03 với coupon; lock ordering mới không được tạo deadlock không xử lý hoặc overselling.
- Outbox order/payment/inventory vẫn ghi theo contract; notification yêu cầu được suy ra từ event nghiệp vụ ở Phase 05, không để hai nguồn cùng tạo thông báo trùng. Chỉ thêm event coupon/review nếu có consumer/use case cụ thể được ghi rõ.
- Cập nhật ERD/OpenAPI/decision log/progress và seed dữ liệu coupon/review có quan hệ Delivered thật.
- Phụ thuộc: P04-01–05.

## 4. Dữ liệu, lỗi và rollback

Coupon preview có thể cũ; checkout là nơi quyết định cuối. Không sửa giá/discount của order đã tạo khi coupon thay đổi. Migration ưu tiên bổ sung tương thích với order cũ; giữ usage/review audit khi disable/ẩn. Rollback app phải kiểm tra schema và dữ liệu mới, không xóa usage hoặc order để “sửa” sai lệch. Log coupon conflict/moderation theo ID và actor, không ghi nội dung nhạy cảm không cần thiết.

## 5. Tiêu chí hoàn thành

- [ ] Coupon đủ chính sách, được kiểm tra lại trong checkout và không vượt usage limit khi đồng thời.
- [ ] Review đúng quyền/điều kiện Delivered, không trùng và moderation/rating public nhất quán.
- [ ] Dashboard/admin/customer UI dùng API/dữ liệu thật; công thức chỉ số được ghi rõ.
- [ ] Regression checkout/payment/stock cũ và có coupon đều đạt.
- [ ] Migration/contract/seed/docs và evidence khớp hành vi đã triển khai.

## 6. Checklist review

- [ ] Không có mutation coupon/stock tách khỏi transaction checkout.
- [ ] Chính sách rounding/minimum/expiry/cancel đã ghi rõ, không tự phát sinh stacking/refund.
- [ ] Review visibility, ownership, uniqueness và dashboard query được kiểm tra.
- [ ] Không tạo duplicate ownership hoặc duplicate notification giữa các module.

Tiếp theo: [Phase 05](05-cache-events-workers-plan.md).
