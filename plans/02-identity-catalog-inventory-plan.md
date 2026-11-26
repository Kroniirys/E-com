---
title: "Phase 02 — Tài khoản, danh mục và tồn kho"
artifact: implementation-plan
role: fullstack
status: draft
version: 0.3
owner: "datnguyen <dat01687241584@gmail.com>"
approvers: []
last_updated: 2026-11-26
---

# Phase 02 — Tài khoản, danh mục và tồn kho

## 1. Mục tiêu và đầu vào

Tạo auth, quản lý người dùng/hàng hóa và UI thật. Đầu vào: [plan tổng](00-master-plan.md), contract/thiết kế và nền tảng đạt tiêu chí [Phase 01](01-foundation-and-design-plan.md).

## 2. Phạm vi

Customer đăng ký/đăng nhập/profile/address và duyệt hàng. Admin quản lý user/product/category, ảnh qua URL, stock/history. Redis rate limit dùng thật; product/category cache và checkout thuộc phase sau.

## 3. Công việc

### P02-01 — Schema và seed

- Phạm vi file: `was/migrations/`, `was/internal/bootstrap/`, repositories auth/users/products/categories/inventory.
- Thực hiện: users/roles/auth_sessions/refresh_tokens/user_addresses, products/product_images/categories/product_categories, inventory/inventory_transactions theo ERD.
- Ràng buộc email/SKU unique và normalize nhất quán; stock không âm, giá hợp lệ, FK/index; inventory là nguồn stock duy nhất.
- Seed admin/customer và catalog đủ Draft/Active/Inactive/hết hàng, lấy thông tin tài khoản demo từ cấu hình local.
- Kiểm tra: DB trống và DB Phase 01 migrate được, seed lặp không trùng và không ghi đè dữ liệu nghiệp vụ cần giữ.
- Phụ thuộc: Phase 01.

### P02-02 — Auth, session, RBAC và rate limit

- Phạm vi file: `was/internal/modules/auth/`, `was/internal/httpapi/`, Redis config.
- Thực hiện: register/login/refresh/logout/change-password/me; bcrypt theo tham số ghi decision log; validate và giới hạn đầu vào.
- JWT kiểm tra thuật toán/signature/expiry. Refresh token ngẫu nhiên lưu hash trong DB, rotation nguyên tử, phát hiện reuse; khóa tài khoản/đổi mật khẩu thu hồi session và chặn access theo chính sách đã thiết kế.
- Customer không tự cấp role Admin. Chặn vô hiệu hóa admin hoạt động cuối cùng.
- Redis rate limit thực tế cho login/register/refresh theo cấu hình; Redis lỗi thì endpoint auth nhạy cảm trả lỗi dịch vụ có kiểm soát, không âm thầm bỏ bảo vệ. Chốt proxy/IP tin cậy.
- Kiểm tra: login đúng/sai, JWT hết hạn, refresh đồng thời/reuse, logout, đổi mật khẩu, tài khoản khóa, customer gọi admin và lỗi Redis.
- Phụ thuộc: P02-01.

### P02-03 — User, profile và address

- Phạm vi file: `was/internal/modules/users/`, routes `/me` và admin users theo OpenAPI.
- Thực hiện: customer sửa profile/CRUD địa chỉ của mình; admin CRUD/search/pagination/status user. “Xóa” là vô hiệu hóa/soft delete để bảo toàn liên kết order ở phase sau.
- Chặn mass assignment role/status; kiểm tra ownership phía server và không trả password hash/token.
- Kiểm tra: IDOR giữa hai user, địa chỉ sai, đổi role trái phép, user khóa và sort/pagination ổn định.
- Phụ thuộc: P02-02.

### P02-04 — Catalog và tìm kiếm

- Phạm vi file: `was/internal/modules/catalog/` (product/category dùng chung transaction).
- Thực hiện: admin CRUD product/category; public list/detail; SKU, ảnh URL, giá bán/giá gốc, mô tả và trạng thái. Out of stock được suy ra từ inventory.
- Search/filter category/sort/pagination có allowlist và giới hạn; không đưa raw input vào SQL. Draft/Inactive không lộ ở public; Active hết hàng vẫn xem được.
- Archive/soft delete giữ tham chiếu; category delete không tạo orphan/FK lỗi. Validate URL và không để server tự fetch URL ngoài nếu chưa kiểm soát SSRF.
- Kiểm tra: SKU trùng, giá sai, pagination biên, public visibility, update rồi reload đọc đúng dữ liệu.
- Phụ thuộc: P02-01/02.

### P02-05 — Stock adjustment và history

- Phạm vi file: `was/internal/modules/inventory/`, admin inventory/history/adjustments.
- Thực hiện: điều chỉnh delta trong transaction có khóa row, chặn kết quả âm, lưu actor/lý do/reference/số trước-sau. Stock ban đầu đi qua nghiệp vụ inventory, product update không ghi stock trực tiếp.
- Bảo vệ adjustment gửi lại bằng idempotency record unique theo actor/operation/key; record và mutation cùng transaction; cùng key khác payload trả 409. Checkout tái sử dụng cơ chế ở Phase 03.
- Kiểm tra: hai adjustment đồng thời, double submit, giảm quá tồn và rollback không sinh history mồ côi.
- Phụ thuộc: P02-01/02/04.

### P02-06 — UI customer/admin

- Phạm vi file: `viewer/src/` gồm API client, auth state, routes và form.
- Customer: Home/listing/detail/search/filter, register/login, profile/address. Home lấy catalog thật, chưa hiển thị checkout như chức năng đã hoạt động.
- Admin: user/product/category CRUD, quản lý image URL, inventory/history/adjustment.
- Có responsive, loading/empty/error, form validation, toast, pagination, keyboard/focus; route guard đi cùng quyền server.
- Thực hiện token refresh có kiểm soát, tránh vòng lặp 401 và xóa session state khi logout.
- Kiểm tra: đi từ UI đến DB, reload giữ phiên đúng chính sách, customer không mở được admin; lỗi network/form phục hồi được.
- Phụ thuộc: P02-02–05.

### P02-07 — Kiểm chứng và tài liệu

- Unit: validation, token policy/RBAC, product status và inventory logic.
- Integration PostgreSQL thật: rotation, uniqueness/FK, ownership, adjustment/idempotency transaction. Redis test bao phủ hoạt động và ngừng dịch vụ.
- Chạy build Go/frontend, `go test ./...`, integration và E2E register → login → admin tạo hàng → customer tìm thấy → admin đổi tồn.
- Đầu ra: OpenAPI/ERD/migration, README và progress/evidence cập nhật.
- Phụ thuộc: P02-01–06.

## 4. Dữ liệu, lỗi và rollback

Không hard delete user/product đã có quan hệ nghiệp vụ. Inventory history/idempotency/mutation phải commit cùng nhau. Không log thông tin nhạy cảm. Nâng cấp migration trước app; rollback app chỉ khi schema tương thích, dữ liệu cần giữ dùng backup hoặc sửa tiến. Không reset volume để che lỗi migration. Catalog đọc PostgreSQL vì cache chưa thuộc phase này.

## 5. Tiêu chí hoàn thành

- [x] Customer/admin dùng UI thật với đúng quyền và session policy.
- [x] Profile/address/user CRUD/status và ownership có kiểm chứng.
- [x] Product/category/search/filter/sort/pagination/status/ảnh dùng dữ liệu thật.
- [x] Stock không âm, adjustment không trùng và history đối chiếu được.
- [x] Redis rate limit và hành vi khi Redis lỗi được kiểm chứng.
- [x] Build/unit/integration/E2E đạt và docs/evidence cập nhật.

## 6. Checklist review

- [x] UI và API contract khớp, đã truy vết call site/error path.
- [x] Không lọt quyền, token, secret hoặc dữ liệu user nhạy cảm.
- [x] Migration, race, idempotency, seed và rollback được kiểm tra.
- [x] Không báo cart/checkout/cache catalog đã hoàn thiện.

Tiếp theo: [Phase 03](03-cart-orders-payments-plan.md).

## Implementation Result

### Lần 1 (2026-11-26)

Đã hoàn thành P02-01–07 trên native PostgreSQL/Redis/Chrome, có review độc lập và sửa lặp. Schema v2, auth/session/RBAC, users/address, catalog và inventory có UI/API chạy thật. Contract 62 operations, 36 đã hiện thực; phần còn lại dành phase sau. Status `draft` giữ nguyên, checkbox thể hiện kết quả triển khai, không phải phê duyệt tài liệu.

Kiểm chứng: 10 Go unit tests dưới race detector, Go vet, 12 integration tests, 9 Viewer unit/component tests, OpenAPI/lint/typecheck/build và 2 browser E2E đều đạt. [Progress/evidence](../docs/PROGRESS.md), [review](../docs/review-phase-02.md), [checklist thực thi](../docs/phase-02-execution.md). Docker tạm hoãn theo yêu cầu chủ dự án. Phase 03 chưa bắt đầu.
