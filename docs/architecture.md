# Kiến trúc E-commerce Base

## Thành phần và ranh giới

```mermaid
flowchart TD
    UI["React / TypeScript / Tailwind"] --> Proxy["Vite dev proxy hoặc Nginx cùng origin"]
    Proxy --> API["Gin API: middleware → handler → service → repository"]
    API --> PG["PostgreSQL / GORM"]
    API --> Redis["Redis: rate limit hiện tại; cache Phase 05"]
    PG --> Outbox["Outbox relay, Phase 05"]
    Outbox --> MQ["RabbitMQ, Phase 05"]
    MQ --> Worker["Consumer / notification, Phase 05"]
    Worker --> PG
```

`was/cmd/api`, `was/cmd/worker`, `was/cmd/migrate`, `was/cmd/seed` là composition root. Config/database/HTTP/runtime dùng chung nhưng không chứa logic giỏ hàng/đơn hàng. Module mới nằm ở `was/internal/modules/<domain>/` khi phase của nó bắt đầu; không tạo hàng loạt repository/service rỗng.

Mỗi module sở hữu bảng và service của mình. Orders điều phối transaction bằng handle DB chung, gọi inventory/payment/coupon với cùng transaction; không gọi HTTP nội bộ, không ghi thẳng bảng module khác từ handler. Các interface chỉ thêm ở boundary có nhu cầu kiểm thử hoặc thay thế thật.

## Phase 01 thực sự cung cấp

- Liveness `/health/live`, readiness `/health/ready` kiểm DB/schema/seed.
- `/api/v1/foundation` đọc cấu hình cửa hàng thật từ PostgreSQL, trả trạng thái nền tảng.
- `/openapi.json` cung cấp contract hợp lệ, gồm endpoint hiện thực và tương lai có đánh dấu.
- React hiển thị kết nối thật, lỗi/mất kết nối và nút kiểm tra lại. Swagger UI được bundle local ở `/api-docs/index.html`, không phụ thuộc CDN.
- Worker bootstrap DB/schema rồi chờ SIGTERM; chưa đăng ký consumer, chưa xử lý notification.

## Khởi chạy, context và lỗi

Migration service chạy riêng rồi seed service. API chỉ ready khi schema version đúng, không dirty và store settings đã seed. Liveness không gọi DB. DB lỗi sau startup làm readiness và foundation lỗi; process còn sống để phục hồi.

HTTP có read-header/read/write/idle timeout, giới hạn body, request ID, security headers, request log cấu trúc và recovery không lộ stack trace. Context timeout truyền vào GORM. Shutdown nhận SIGINT/SIGTERM, ngừng listener, drain request rồi đóng pool DB. Nếu vượt shutdown timeout, buộc đóng và ghi lỗi.

Frontend dùng cùng origin; dev proxy/Nginx chuyển `/api`, `/health`, `/openapi.json`. Backend CORS allowlist chính xác cho trường hợp gọi trực tiếp; không wildcard credentials. Phase 02 yêu cầu `AUTH_JWT_SECRET` tối thiểu 32 byte và Redis URL tại API startup. Các lệnh migrate/seed/worker chỉ đọc cấu hình cần thiết cho vai trò của mình.

## Các lỗi thiết kế đã xử lý

- Không dùng Redis cho stock hoặc idempotency giao dịch.
- Không publish broker trước/sau commit mà thiếu outbox.
- Không coi attempt Failed là order Failed; xem [state machine](order-payment-state-machine.md).
- Không trả thông tin DSN/SQL/panic cho client; log bootstrap chỉ có thông tin lỗi đã làm sạch.
- Không tự migrate trong API hoặc seed phá dữ liệu mỗi lần restart.

## Tài liệu liên quan

[ERD](erd.md), [API](api-design.md), [event contract](events.md), [quyết định](architecture-decisions.md), [requirements](requirements.md), [tiến độ](PROGRESS.md).

## Bố trí repository

Backend nằm hoàn toàn trong `was/` với module Go riêng. `was/api/openapi.yaml` là nguồn chuẩn contract, được embed qua `was/api/embed.go` vào API binary. Frontend nằm trong `viewer/`, chỉ đọc contract này để sinh TypeScript; không phụ thuộc package Go hoặc tài liệu Markdown khi build. Docs/plans ở root dùng chung và không nằm trong module Go.

Root Makefile gọi `make -C was` và `npm --prefix viewer`. Runner E2E ở root dùng binary `was/bin/` và Vite từ `viewer/`. Backend test runner ở `was/scripts/`. Dockerfile backend dùng context `was/`; Dockerfile viewer dùng context root để lấy contract `was/api/`. Kiểm chứng Docker đang được hoãn theo yêu cầu người dùng.

## Phase 02 đã hiện thực

`was/internal/httpapi` gắn route/validation/RBAC vào HTTP server; `modules/auth`, `modules/users`, `modules/catalog`, `modules/inventory` sở hữu nghiệp vụ. Catalog gộp product/category vì assignment cần transaction chung. `platform/problem` thống nhất lỗi và `platform/limit` thực thi Lua rate limit trên Redis. Không có HTTP nội bộ giữa module.

JWT HS256 chứa session ID/version, được đối chiếu user/session trong PostgreSQL. Refresh cookie HttpOnly/SameSite=Lax, kiểm Origin ở cookie mutation; token chỉ lưu hash, rotation/replay có khóa và transaction. Redis lỗi làm auth nhạy cảm trả 503; catalog tiếp tục đọc DB. Không tin header forwarded IP mặc định. Xem [API design](api-design.md) và [decision log](architecture-decisions.md).

Viewer có catalog/detail, đăng ký/đăng nhập, tài khoản/địa chỉ/password và admin users/products/categories/inventory. `SessionRuntime` quản lý access token trong memory; Web Locks serialize cookie mutation giữa tab, BroadcastChannel đồng bộ đổi phiên. Auth epoch ngăn request cũ replay hoặc phát dữ liệu sang tài khoản vừa đăng nhập. `Field` dùng label liên kết ID rõ ràng; loading/error/empty/validation được dùng chung trong `ui.tsx`.

Integration tạo schema PostgreSQL và prefix Redis riêng; E2E runner sinh secret tạm và schema riêng qua `cmd/testscope`. Cleanup chỉ tài nguyên thuộc lần chạy. Worker vẫn chưa consume sự kiện; cache catalog và publisher/consumer thuộc các phase sau.

## Phase 03 đã hiện thực

`modules/cart` sở hữu carts/items và live quote; `modules/orders` điều phối transaction checkout/lifecycle cùng `modules/payments` (aggregate/attempt) và `inventory.Change`. `platform/money` chặn tiền vượt miền, `platform/idempotency` claim/result/retry, `platform/outbox` ghi sự kiện bền. Không gọi broker hoặc gateway trong transaction.

Checkout dùng user NO KEY UPDATE → cart → product và stock theo ID sau khi claim idempotency. Lifecycle/payment khóa order trước payment/attempt/stock. Read detail/list dùng snapshot RepeatableRead để tránh ghép status/payment từ hai thời điểm. Service và DB partial unique cùng bảo vệ Pending/Paid attempt; admin/customer transition tách namespace replay.

Viewer thêm cart/checkout/orders/admin orders và intent recovery trong sessionStorage theo user. Không persist token; request vẫn chịu auth epoch guard. Khi response không rõ kết quả, giữ key để kiểm tra lại cả sau reload. Khi quote thay đổi, tải lại và yêu cầu xác nhận mới.

HTTP log có request/order/attempt/error code. Outbox log ghi event ID và correlation ở mức staged, không khẳng định transaction đã commit hoặc event đã publish. Nguồn chuẩn về event đã commit là outbox trong PostgreSQL; publisher và metrics thuộc Phase 05.

## Phase 04

Modules coupons/reviews/admin được wiring qua httpapi/engagement. Orders điều phối coupon row lock giữa product và stock, lưu snapshot/usage cùng transaction; không gọi mạng hoặc consumer. Review và dashboard đọc snapshot RepeatableRead. Dashboard sum là decimal string tới Viewer BigInt; owner quản lý review riêng ở tài khoản. Không phát thêm event coupon/review khi chưa có consumer.

## Phase 05

API catalog có Redis cache adapter và DB-generation snapshot; checkout vẫn đọc PostgreSQL. Producer ghi outbox tại module gốc. Worker là process thật, relay và hai consumer purpose, mỗi purpose main/retry receiver bounded. AMQP publisher dùng gate context-aware, deadline handshake/write/confirm, mandatory returns và confirms; SQL không giữ transaction khi publish hoặc gọi Redis.

Notifications module cung cấp list/unread/read theo owner. Worker readiness kiểm broker/DB/Redis, API readiness không phụ thuộc broker; Prometheus scrape metrics API/worker trên mạng local/internal. Compose đã được kiểm chứng thực tế, không còn ở trạng thái tạm hoãn. [Operations](operations.md).
