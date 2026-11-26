# Checklist thực thi Phase 05

Đầu vào Phase04 đã merge develop848579d. Docker đã được người dùng yêu cầu kiểm tra trở lại, truy cập được qua sg docker -c. Dùng project/volume/port riêng, không purge dữ liệu có sẵn. Plan fullstack E-com là đầu vào được giao; không áp gate projects.toml/approved artifact của TeemStone.

| Bước | File | Công việc và kiểm chứng |
| --- | --- | --- |
| 1 | migrations/000005_*, platform/cache | Schema5 generation/inbox/notification/claim; cache key theo DB generation, snapshot read và timeout/fallback bound |
| 2 | modules/catalog, inventory | Producer product/category/inventory outbox cùng transaction; deferred generation trigger chống late-fill |
| 3 | infrastructure/rabbitmq, worker | Topology durable, mandatory/confirm, claim lease token, consumer inbox/owner, retry queue có manual ack và DLQ |
| 4 | cmd/worker, cmd/events | Reconnect/shutdown, readiness/metrics, inspect/replay giữ event ID |
| 5 | modules/notifications, httpapi, Viewer | Owner list/unread/mark-read, lịch sử sự kiện không đảo trạng thái order |
| 6 | metrics, Compose, monitoring, scripts | Prometheus thực, stack mặc định và kiểm thử Docker/fault recovery trên volume riêng |
| 7 | integration/E2E/docs | Concurrency/duplicate/lease/crash/routing/cache race, review độc lập, sửa lặp và raw evidence |

Generation nguồn chuẩn ở PostgreSQL, bump bằng constraint trigger deferred sau mọi thay đổi catalog/inventory. Reader mở RepeatableRead, lấy generation rồi tra cache key theo generation; late-fill chỉ ghi key cũ và request mới không dùng. Consumer invalidation quan sát generation hiện tại bằng MAX trong Redis, không hạ generation. Redis restart không làm DB generation lùi. Metadata public lấy cùng DB snapshot; admin không dùng cache public. TTL detail60s/list30s, Redis timeout100ms, giới hạn16 fallback đồng thời.

Publish một message trên channel riêng được mutex serialize: mandatory return và confirm đều phải thành công mới mark published. Claim1row, lease10s, publish timeout3s; mark/release kiểm token và lease. Retry consumer hữu hạn3lần qua queue durable có not-before; không phụ thuộc dead-letter TTL at-most-once. Publish retry/DLQ được confirm+mandatory trước ack gốc; reconnect đóng channel để requeue unacked, không hot nack loop. Poison schema/version sang DLQ. CLI replay reset retry nhưng giữ event ID.

## Implementation Result — Lần 1 (2026-11-26)

Hoàn thành bước1–7 với review độc lập và sửa lặp. 19Go unit/race,39integration,13Viewer unit,5nativeE2E,1Dockerbrowser và9Dockerchecks đạt. Bằng chứng và phạm vi tại [progress](PROGRESS.md). Không có publisher/email thật hoặc bypass idempotency; stock/payment/coupon vẫn ở transaction đồng bộ. Không xóa queue/volume ngoài namespace test. Phase05 chưa commit/push.
