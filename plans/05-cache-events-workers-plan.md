---
title: "Phase 05 — Cache, sự kiện và worker"
artifact: implementation-plan
role: fullstack
status: draft
version: 0.4
owner: "datnguyen <dat01687241584@gmail.com>"
approvers: []
last_updated: 2026-11-26
---

# Phase 05 — Cache, sự kiện và worker

## 1. Mục tiêu và đầu vào

Hoàn thiện Redis cache, reliable async processing và notification có thể phục hồi. Đầu vào: [plan tổng](00-master-plan.md), [Phase 04](04-coupons-reviews-admin-plan.md) đạt tiêu chí; outbox đã được tạo cùng giao dịch từ Phase 03/04 và rate limit Redis đã dùng từ Phase 02.

## 2. Phạm vi

Product/category cache, invalidation/fallback; outbox relay, RabbitMQ, consumer/retry/DLQ; notification nội bộ/mock email; Prometheus và toàn stack Compose. Không chuyển transaction stock/payment/coupon sang worker, không tách microservices hoặc thêm Kubernetes.

## 3. Công việc

### P05-01 — Cache product/category và invalidation

- Phạm vi file: Redis adapter, products/categories read service và config.
- Cache-aside cho product/category detail và list theo key có version/filter hash; TTL/config được ghi rõ. Không dùng cache làm nguồn giá/stock ở checkout hoặc quyết định quyền/status.
- Invalidate detail và list khi product/category/image/status/stock thay đổi. Thiết kế generation/version và invalidation bền qua outbox để xử lý race “read cũ ghi cache sau invalidate”, không chỉ xóa một key rồi coi xong.
- Bổ sung producer `product.changed` và `category.changed` cùng transaction cập nhật catalog; consumer invalidation nhận các event này và `inventory.updated`. Đăng ký event version/schema trong contract; không chỉ nối consumer mà bỏ sót mutation ở module nguồn.
- Public visibility phải kiểm tra nguồn chuẩn trước trả dữ liệu cache để Draft/Inactive không tiếp tục lộ khi invalidation bị trễ. Chốt mức stale được phép cho field không nhạy cảm; không dùng TTL che yêu cầu visibility.
- Redis lỗi thì catalog fallback PostgreSQL; auth rate limit giữ hành vi bảo vệ Phase 02. Giới hạn timeout/fallback concurrency để tránh làm DB quá tải.
- Kiểm tra: hit/miss, detail/list/category update, stock về 0, deactivate/delete, read-write race, Redis restart và fallback; thay cache không đổi tổng checkout.
- Phụ thuộc: Phase 04, phối hợp P05-02 cho invalidation bền.

### P05-02 — Outbox relay và RabbitMQ topology

- Phạm vi file: `was/cmd/worker/`, `was/internal/infrastructure/rabbitmq/`, outbox repository và Compose.
- Envelope event có event ID, type, version, aggregate ID/version, occurred_at, correlation ID và payload tối thiểu, tránh thông tin cá nhân không cần thiết.
- Contract nghiệp vụ: `order.created`, `order.confirmed`, `order.cancelled`, `payment.completed`, `inventory.updated`; bổ sung status-changed event cho Processing/Shipped/Delivered/Failed theo state machine, cập nhật producer cùng transaction ở module sở hữu.
- `notification.requested` chỉ dùng làm internal job khi cần tách consumer, không tạo thêm nguồn notification độc lập cho cùng event. Ghi mapping event → consumer/purpose rõ ràng.
- Durable exchange/queue, persistent message, publisher confirms và xử lý unroutable/mandatory return; chỉ mark published khi broker đã xác nhận và route hợp lệ.
- Relay claim batch với row lock/lease an toàn cho nhiều worker; lease hết được reclaim. Lỗi publish giữ record pending để retry, crash sau publish trước mark có thể phát lại cùng event ID.
- Kiểm tra: broker down, publish timeout/unroutable, restart relay, hai relay cùng chạy và crash ở các khoảng lỗi; không mất outbox hoặc giữ claim vĩnh viễn.
- Phụ thuộc: outbox Phase 03/04; topology trước P05-03.

### P05-03 — Consumer, idempotency, retry và DLQ

- Phạm vi file: worker consumer, processed-events repository, retry/DLQ configuration.
- Manual ack sau khi side effect DB commit. Unique `(consumer,event_id)` và cập nhật notification/processed event trong cùng transaction; duplicate delivery không tạo side effect thêm.
- Phân loại transient/permanent/invalid schema; retry hữu hạn có backoff và attempt metadata, không nack/requeue vô hạn. Sau giới hạn chuyển DLQ; chỉ ack bản gốc sau khi đường retry/DLQ đã nhận an toàn.
- Chốt per-aggregate ordering/version behavior; event cũ không được đảo trạng thái hiển thị, không giả định thứ tự RabbitMQ khi có retry/nhiều worker.
- Graceful shutdown ngừng nhận, hoàn tất hoặc để unacked requeue; có prefetch/concurrency giới hạn. Công cụ inspect/replay DLQ giữ event ID để dedupe.
- Kiểm tra: duplicate, poison message, DB down, kill worker trước/sau commit và trước ack, retry hết hạn mức, replay DLQ và nhiều consumer.
- Phụ thuộc: P05-02.

### P05-04 — Notification và UI

- Phạm vi file: `was/internal/modules/notifications/`, notifications migration, customer UI và API.
- Tạo notification khi order tạo/thay trạng thái và payment hoàn thành theo mapping duy nhất; ownership user cố định từ dữ liệu tin cậy.
- API list/pagination/unread/mark-read, customer chỉ đọc/sửa notification của mình; notification xuất hiện theo eventual consistency, lỗi gửi không rollback order đã commit.
- Mock email ra console được ghi rõ là mô phỏng; DB notification chống trùng. Log mock email có thể lặp sau crash, không hứa exactly-once với side effect ngoài DB; nếu thêm email thật cần cơ chế idempotency riêng.
- UI notification list/unread/read, có loading/empty/error và refresh; chưa tự thêm WebSocket/push requirement.
- Kiểm tra: order success → event → notification UI, status update, duplicate không tạo thêm bản ghi và IDOR giữa customer.
- Phụ thuộc: P05-03.

### P05-05 — Monitoring và cấu hình toàn stack

- Phạm vi file: metrics instrumentation, Prometheus config, Dockerfile/Compose, README.
- Metrics HTTP request/error/latency, cache hit/miss/fallback, outbox pending/age, relay failures, queue retry/DLQ, consumer outcome và DB pool. Không dùng order/user/event ID làm metric label.
- Structured log theo request/event/order ID; không log secret/payload nhạy cảm. Metrics/management endpoint không mở public ngoài nhu cầu local.
- Đưa app/worker/postgres/redis/rabbitmq/frontend vào đường chạy mặc định; Prometheus có cấu hình scrape thực tế. Migration là bước riêng chạy đúng thứ tự, readiness phản ánh dependency bắt buộc và chế độ suy giảm.
- Dữ liệu PostgreSQL/RabbitMQ dùng volume; API phục vụ được nghiệp vụ khi broker down nhờ outbox, worker readiness báo lỗi; Redis down tuân theo fallback catalog và auth policy.
- Kiểm tra: `docker compose up --build` chạy toàn stack, service restart không mất dữ liệu, Prometheus thu metrics và log có thể trace một order qua event.
- Phụ thuộc: P05-01–04.

### P05-06 — Fault injection, regression và tài liệu

- Test tích hợp dùng Redis/RabbitMQ/PostgreSQL thật; ngắt từng dependency, kill/restart worker, backlog recovery, duplicate/out-of-order, retry/DLQ/replay và cache race.
- Dừng broker → tạo order → kiểm tra outbox pending → bật broker/worker → notification cuối cùng xuất hiện đúng một bản ghi. Chờ có timeout kiểm tra theo chính sách cấu hình, không polling vô hạn.
- Chạy lại luồng checkout/coupon/payment/stock; async lỗi không đổi kết quả giao dịch đã commit. Không để event history cũ bị phát email thật khi replay seed.
- Đầu ra: `docs/events.md`, `docs/operations.md`, cấu hình metrics, evidence failure/recovery, OpenAPI/progress/known issues cập nhật.
- Phụ thuộc: P05-01–05.

## 4. Dữ liệu, lỗi và rollback

Mọi event producer giữ outbox cùng transaction nghiệp vụ. Không purge queue/DLQ/outbox để làm sạch test trên dữ liệu cần giữ. Khi consumer lỗi, dừng consumer và giữ backlog; rollback chỉ sang phiên bản hiểu schema/event đang tồn tại, nếu không dùng sửa tiến. Cache có thể tắt/bypass bằng config và vẫn dùng PostgreSQL; giới hạn dữ liệu stale phải được kiểm chứng. Outbox/processed-event retention không xóa dữ liệu còn cần replay/dedupe.

## 5. Tiêu chí hoàn thành

- [x] Redis phục vụ cache và rate limit thực tế; invalidation/race/fallback có kiểm chứng.
- [x] Outbox relay có confirm/route handling, recovery và chống claim kẹt.
- [x] Consumer idempotent, retry hữu hạn, DLQ/replay và shutdown hoạt động.
- [x] Notification từ event hiện trên UI đúng quyền; mất broker/worker không mất sự kiện đã commit.
- [x] Compose toàn stack chạy được và Prometheus thu dữ liệu thật.
- [x] Fault injection/regression đạt, docs và evidence đủ tái lập.

## 6. Checklist review

- [x] Phân biệt atomic transaction với at-least-once delivery; không hứa exactly-once ngoài DB.
- [x] Đã kiểm tra crash windows, routing failure, duplicate, ordering và retry poison message.
- [x] Cache không làm lộ hàng bị ẩn hoặc ảnh hưởng giá/stock checkout.
- [x] Metrics/log/queue management không lộ secret hoặc tạo cardinality cao.

Tiếp theo: [Phase 06](06-verification-handover-plan.md).

## Implementation Result

### Lần 1 (2026-11-26)

Đã hoàn thành P05-01–06: schema5, cache generation/visibility/fallback, producer outbox đầy đủ, relay lease/token và mandatory/confirms, inbox/notification, bounded retry/DLQ/replay, UI, metrics/Prometheus và Compose mặc định. Không chuyển stock/payment/coupon sang worker.

Kiểm chứng:19 Go unit/race +vet,39 integration trên PostgreSQL/Redis/RabbitMQ thật,13 Viewer unit và5E2E native; thêm1browserE2E qua Nginx Docker và9fault/recovery checks Docker. Có fake handshake, cache hit/late-fill, hai relay, stale token, publish-before-mark, commit-before-ack/redelivery, retry hết hạn mức/DLQ/replay. Review độc lập đã sửa handshake timeout và nội dung notification thanh toán; không còn finding cần sửa trong phạm vi đã rà.

[Progress/evidence](../docs/PROGRESS.md), [review](../docs/review-phase-05.md), [operations](../docs/operations.md). Docker đã kiểm thực tế theo yêu cầu mới. Status giữ draft, không tự phê duyệt artifact. Phase06 chưa bắt đầu; Phase05 chưa commit/push.
