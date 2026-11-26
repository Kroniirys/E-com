# Vận hành Phase 05

## Khởi chạy

Dùng scripts/setup-local.py để tạo .env riêng, không commit file này. Phase05 cần RABBITMQ_PASSWORD và RABBITMQ_URL ngoài cấu hình Phase04; với .env cũ hãy bổ sung hai giá trị tương ứng bằng cách sửa local. URL native dùng amqp://ecom:<password>@localhost:5672/; Compose tự dùng host rabbitmq và password từ .env. Mật khẩu thay đổi không tự đổi credential trong RabbitMQ volume đã khởi tạo; không xóa volume để che lỗi credential.

Chạy docker compose up -d --build. Trong phiên Codex hiện tại, dùng sg docker -c 'docker compose …' vì membership docker chưa có hiệu lực trực tiếp trên process cũ. Không chmod socket, không sửa daemon hoặc thêm sudo flow. Stack gồm PostgreSQL → migrate → seed → API, Viewer, Redis, RabbitMQ, worker và Prometheus. API không phụ thuộc broker để checkout; worker tự reconnect. Các port publish chỉ bind127.0.0.1, metrics không proxy qua Viewer.

Native worker cần DATABASE_URL/schema5, REDIS_URL, RABBITMQ_URL, BROKER_PREFIX và WORKER_HTTP_ADDR (mặc định8082). Production bắt buộc amqps/rediss; config không ghi URL/secret ra log. CACHE_ENABLED=false bypass API catalog cache; auth rate limit vẫn cần Redis.

## Cache và visibility

Generation nguồn chuẩn là PostgreSQL, được bump bởi deferred constraint trigger trên product/category/image/assignment/inventory trong cùng transaction. Cache reader mở RepeatableRead, lấy generation rồi tra key theo namespace DB và hash đầy đủ filter/sort/page. Late-fill ghi generation cũ không được reader mới dùng; visibility được bảo vệ bởi snapshot và generation nguồn chuẩn kể cả broker chưa invalidate. Redis restart không làm DB generation lùi. TTL detail60s/list30s, Redis operation100ms, tối đa16fallback hydrations đồng thời/process; generation query vẫn dùng pool/context timeout.

Invalidator nhận product.changed/category.changed/inventory.updated, quan sát DB generation hiện tại và ghi MAX vào Redis trước inbox commit. Không gọi Redis khi giữ SQL transaction. Retry/duplicate có thể thực hiện Redis MAX lặp, nhưng không làm state lùi. Khi restore DB hoặc đổi dataset, đổi namespace/URL hoặc xóa cache có kiểm soát trước mở traffic để không tái dùng generation từ bản backup cũ.

## Outbox, retry và notification

Claim dùng SKIP LOCKED, một record mỗi lượt, lease10s và token mới. Publish ngoài SQL transaction, gate serialize một message/channel, deadline3s bao gồm chờ gate + socket write + confirm. Mandatory return dù có positive confirm vẫn là thất bại. Mark/release đối chiếu claim token; stale worker không mark claim mới. Lỗi publish giữ pending và next_attempt_at backoff tối đa30s; crash sau publish trước mark phát lại cùng event ID.

Topology durable: <prefix>.events, hai queue <prefix>.notifications.v1 và <prefix>.catalog.v1; mỗi queue có .retry và .dlq. Persistent publish; manual ack sau SQL commit hoặc sau retry/DLQ publish được confirm và route hợp lệ. Retry queue do consumer quản lý delay1s, tối đa3lần; không dùng TTL dead-letter at-most-once để chuyển retry. Poison/unknown version vào DLQ ngay. Broker/connection lỗi đóng channel để unacked requeue và reconnect, không hot nack loop.

Consumer ghi processed_events cùng notification trong một transaction; owner được đối chiếu order/payment trong DB. Duplicate hoặc crash sau commit trước ack không tạo thêm notification. UI hiển thị lịch sử theo occurred_at/version, không overwrite trạng thái order bằng event cũ. Mock email chỉ log ID; không gửi email thật và không hứa exactly-once cho console.

## Inspect và replay

Chạy was/bin/events inspect notifications hoặc catalog để đọc metadata một message DLQ rồi trả về queue. Chạy was/bin/events replay notifications hoặc catalog để chuyển một message về queue chính, giữ event ID và reset retry. Chỉ ack DLQ sau publish mới đã confirm/mandatory thành công. Không purge queue/outbox để xử lý lỗi. cleanup-test chỉ dùng APP_ENV=test với namespace ecom_e2e_<32hex> do runner tạo, không áp lên queue vận hành.

## Health và metrics

API /health/ready phản ánh DB/schema/seed. Worker /health/ready cần broker session, PostgreSQL và Redis; broker down không làm API rollback order. /internal/metrics trên API và worker có HTTP request/duration sum/count theo route template, cache hit/miss/fallback/invalidation, relay publish/failure, consumer success/duplicate/retry/DLQ, outbox pending/age và DB pool. Không có user/order/event ID trong metric label. Prometheus scrape hai job với interval5s; counter reset khi process restart là bình thường.

Log nối request/order/attempt/event ID; outbox staged không đồng nghĩa đã commit/publish. Publisher log outbox published sau mark; consumer mock email không chứa địa chỉ/password/token. Metrics, broker management và test endpoints chỉ dùng mạng local/internal, chưa có deployment public.

## Kiểm chứng và phục hồi

scripts/verify-phase05-docker.py --env-file <private env> --project ecom-phase05-verify chỉ chạy project test cùng prefix ecom-phase05-. Script kiểm Nginx proxy, broker stop/recovery, duplicate, SIGKILL worker/backlog, Redis fallback/auth503/checkout nguồn DB, PostgreSQL restart, full Compose restart giữ volume/read-state và Prometheus up/runtime samples. Không chạy script này lên stack chứa dữ liệu cần giữ.

Integration dùng database *_test/schema riêng, TEST_REDIS_URL và TEST_RABBITMQ_URL; mọi topology AMQP test có namespace random và cleanup riêng. E2E khởi chạy API + worker cho từng spec, đồng bộ readiness/version, dọn process/schema/topology test. Khi cần browser qua container, đặt E2E_EXTERNAL_SERVER=true và E2E_BASE_URL đúng Viewer URL.

Sau lỗi: phục hồi dependency, chờ pending/age và retry giảm; inspect DLQ và nguyên nhân trước replay. Khi đổi event schema không tương thích, giữ worker cũ hoặc viết migration/adapter; không xóa inbox/outbox còn cần dedupe. Down migration5 từ chối khi có inbox hoặc notification; dữ liệu thật dùng sửa tiến.

Tham khảo transport: [AMQP client chính thức RabbitMQ](https://pkg.go.dev/github.com/rabbitmq/amqp091-go). Kết quả thực chạy tại [progress](PROGRESS.md).
