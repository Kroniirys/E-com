# E-commerce Base

Workspace tách rõ hai phần: **WAS là backend Go**, **Viewer là frontend React**. Phase 03 bổ sung giỏ hàng, checkout, đơn hàng và COD/thanh toán mô phỏng trên nền tảng tài khoản/catalog/tồn kho; Docker hiện được tạm hoãn theo yêu cầu.

## Cấu trúc

```text
E-com/
├── was/                       # Backend
│   ├── api/                   # OpenAPI và Go embed
│   ├── cmd/                   # api, worker, migrate, seed, healthcheck
│   ├── internal/              # config, database, httpserver, runtime
│   ├── migrations/            # SQL migration có version
│   ├── tests/integration/     # PostgreSQL integration tests
│   ├── scripts/               # Runner test backend
│   ├── bin/                   # Binary build, không commit
│   ├── go.mod / go.sum
│   ├── Makefile
│   └── Dockerfile
├── viewer/                    # Frontend
│   ├── src/                   # React, client và component tests
│   ├── e2e/                   # Playwright
│   ├── public/api-docs/       # Swagger UI local
│   ├── scripts/               # Chuẩn bị Swagger assets
│   ├── package.json / package-lock.json
│   ├── Dockerfile
│   └── nginx.conf
├── docs/                      # Yêu cầu, thiết kế và evidence dùng chung
├── plans/                     # Plan tổng và 6 phase
├── scripts/run-e2e.py          # Điều phối WAS + Viewer
├── Makefile                   # Điều phối workspace
├── docker-compose.yml
└── .env.example
```

Go module chỉ nằm trong `was/`; chạy lệnh Go trực tiếp từ đó. Npm project chỉ nằm trong `viewer/`. Root Makefile hỗ trợ chạy tất cả các lệnh điều phối từ thư mục gốc.

Contract chuẩn: [was/api/openapi.yaml](was/api/openapi.yaml). WAS embed contract vào binary; Viewer sinh `viewer/src/api.generated.ts` từ cùng file, không giữ bản contract thứ hai. Mã nghiệp vụ của hai phần không phụ thuộc chéo vào nhau.

## Chạy native

Công cụ đã dùng: Go 1.27.4, Node 24/npm và PostgreSQL 17.12. Dependency khóa trong `was/go.mod`, `was/go.sum`, `viewer/package-lock.json`.

Tạo database phát triển và user bằng công cụ PostgreSQL của bạn, sau đó:

```bash
python3 scripts/setup-local.py
npm --prefix viewer ci
```

Script chỉ tạo `.env` mới với quyền đọc/ghi cho chủ file, sinh secret và mật khẩu demo ngẫu nhiên, không ghi đè file cũ hoặc in secret ra terminal. Mở `.env` trên máy để xem tài khoản demo và điều chỉnh `DATABASE_URL`/`REDIS_URL` trỏ đến PostgreSQL/Redis đang chạy. Không commit file này. Khởi tạo và chạy WAS:

```bash
make migrate-up
make seed
make run
```

Terminal khác chạy Viewer:

```bash
make viewer-dev
```

- Viewer: http://localhost:5173
- API WAS: http://localhost:8080
- Giỏ hàng: http://localhost:5173/#/cart
- Đơn hàng: http://localhost:5173/#/orders
- Tài khoản: http://localhost:5173/#/account
- Quản trị: http://localhost:5173/#/admin/products
- Trạng thái nền tảng: http://localhost:5173/#/system
- Swagger qua Viewer: http://localhost:5173/api-docs/index.html
- Health: http://localhost:8080/health/live và /health/ready
- Contract: http://localhost:8080/openapi.json

Vite proxy API cùng origin đến localhost:8080; dùng `API_PROXY_TARGET` nếu WAS chạy địa chỉ khác. Backend không tự nạp `.env` khi gọi binary trực tiếp; Make/Compose nạp environment. Biến truyền trên command line Make ưu tiên cao hơn `.env`:

```bash
make DATABASE_URL='postgres://user:password@localhost:5432/ecom?sslmode=disable' migrate-up seed
```

Có thể thao tác riêng backend:

```bash
make -C was build
make -C was test
make -C was run
```

Makefile của WAS nạp `../.env`; nếu chạy `go run` trực tiếp thì cần cấp environment. Với Go Snap bị hạn chế wrapper, có thể dùng `make GO=/snap/go/current/bin/go ...` khi binary đó tồn tại.

## Kiểm tra và build ngoài Docker

```bash
npm --prefix viewer ci
make check
make build
```

- `make check`: Go unit/vet trong WAS, OpenAPI validate/type generation và Viewer lint/typecheck/unit/build.
- `make build`: binary WAS vào `was/bin/`.
- `make viewer-check`: chỉ kiểm tra Viewer cùng contract.
- `make contract`: validate contract và sinh TypeScript từ `was/api/openapi.yaml`.
- `make format`: format mã Go trong WAS.

Có thể gọi frontend riêng bằng `npm --prefix viewer run ...`. Build/dev tự sinh type và chuẩn bị Swagger assets từ dependency đã khóa, không dùng CDN.

Integration và E2E cần PostgreSQL test thật có tên database kết thúc `_test`:

```bash
export TEST_DATABASE_URL='postgres://user:password@localhost:5432/ecom_test?sslmode=disable'
export TEST_REDIS_URL='redis://localhost:56379/0'
make test-integration
npm --prefix viewer exec -- playwright install chromium
make test-e2e
```

Khi đã truyền `TEST_DATABASE_URL`, runner integration không gọi Docker. Nếu không truyền, runner có chế độ khởi tạo service test bằng Compose; không dùng chế độ này khi đang hoãn kiểm tra Docker.

Integration tạo schema `ecom_it_<random>` và prefix Redis riêng; migrate/seed/HTTP/rollback rồi chỉ dọn dữ liệu của mình. E2E cũng tạo schema `ecom_e2e_<random>`, JWT secret và dữ liệu demo riêng, tự cleanup khi xong. Không dùng DB test để lưu dữ liệu cần giữ. E2E dùng `was/bin/`, khởi chạy API port 18080 và Vite port 5173, xác minh APP_VERSION riêng để không pass nhầm backend. Mỗi spec E2E dùng schema/API/Redis prefix riêng để không chia sẻ hạn mức đăng nhập hoặc dữ liệu. Đóng Vite dev trước khi chạy E2E. Python 3 được dùng cho runner điều phối. Để lưu riêng evidence và giữ kết quả lịch sử, đặt `E2E_EVIDENCE_DIR=docs/evidence/phase-03` khi chạy `make test-e2e`; runner chuẩn hóa đường dẫn và dùng chung cho API log/ảnh browser.

Nếu dùng Chrome sẵn có, đặt `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH` bằng đường dẫn binary. Thiếu browser/library phải ghi chưa kiểm chứng, không coi pass. Trace/report ở `viewer/test-results/`, `viewer/playwright-report/` được ignore. Evidence nền tảng ở `docs/evidence/phase-01/`; evidence lần tách cấu trúc ở `docs/evidence/restructure-was-viewer/`; kết quả Phase 02 ở `docs/evidence/phase-02/`, Phase 03 ở `docs/evidence/phase-03/`.

## Migration và seed

`make migrate-up` dùng golang-migrate đã khóa trong Go module, không cần CLI global. Migration version 1 tạo `store_settings`; version 2 thêm roles/users/auth_sessions/refresh_tokens, addresses, catalog và inventory/idempotency/history; version 3 thêm cart/order/order_items/history, payment/attempts và outbox. `SEED_DEMO_DATA=true` yêu cầu email và mật khẩu riêng cho Admin/Customer từ `.env`, đồng thời tạo catalog mẫu đủ Active/Draft/Inactive/hết hàng. Mật khẩu được bcrypt cost 12, stock ban đầu đi qua nghiệp vụ inventory. Seed chỉ cho development/test, chạy lặp không đổi password/profile/stock/dữ liệu đã có. Đặt `SEED_DEMO_DATA=false` nếu chỉ cần seed cấu hình cửa hàng.

API không tự migrate. Thiếu/sai schema, dirty migration hoặc chưa seed khiến readiness/foundation trả 503; liveness vẫn phản ánh process. Khắc phục DB/schema/seed sẽ phục hồi readiness.

Rollback chỉ dành database test có thể xóa và có đủ xác nhận:

```bash
make TEST_DATABASE_URL='postgres://user:password@localhost:5432/ecom_test?sslmode=disable' ALLOW_TEST_RESET=delete-test-schema reset-test
```

Guard từ chối tên DB khác, URL query ghi đè database và path bất thường. Dữ liệu cần giữ phải backup, kiểm tra tương thích và ưu tiên migration sửa tiến; không reset hoặc xóa volume để che lỗi.

## Docker — tạm hoãn kiểm chứng

Dockerfile/Compose đã được điều chỉnh đường dẫn cho WAS/Viewer. Trong lượt cơ cấu lại này không chạy Docker build, Compose config hoặc kiểm tra runtime Docker. Khi cần quay lại kiểm chứng:

```bash
cp .env.example .env
docker compose up --build
```

Backend build context là `was/`; Viewer build context là root để lấy contract từ WAS. Service Compose vẫn giữ tên `app`/`frontend` để không đổi wiring ngoài yêu cầu. Chuỗi startup vẫn PostgreSQL → migrate → seed → app → frontend. URL mặc định khi chạy Compose: Viewer localhost:8081, API localhost:8080.

`docker compose down` giữ named volume. Redis là dependency mặc định cho auth Phase 02; profile `infrastructure` giữ RabbitMQ/worker. Worker mới bootstrap/shutdown, chưa consume message. Local credentials chỉ là mẫu; password có ký tự đặc biệt trong DB URL phải được URL-encode.

## Tài liệu

- [Plan tổng](plans/00-master-plan.md), [Phase 01](plans/01-foundation-and-design-plan.md), [Phase 02](plans/02-identity-catalog-inventory-plan.md), [Phase 03](plans/03-cart-orders-payments-plan.md)
- [Backend WAS](was/README.md), [Frontend Viewer](viewer/README.md)
- [Requirements](docs/requirements.md), [architecture](docs/architecture.md), [ERD](docs/erd.md)
- [API design](docs/api-design.md), [state machine](docs/order-payment-state-machine.md), [events](docs/events.md)
- [Progress và evidence](docs/PROGRESS.md), [known issues](docs/known-issues.md)

OpenAPI có contract tương lai được đánh dấu rõ. Phase 01–03 hiện thực foundation/auth/profile/address/catalog/admin/inventory/cart/order/payment. Coupon/review/notification thuộc phase sau, chưa đăng ký và trả 404. Storefront và các màn hình quản trị dùng dữ liệu PostgreSQL thật; không có nút checkout giả.

## Chính sách tài khoản Phase 02

Mật khẩu tối thiểu 12 ký tự Unicode, tối đa 72 byte UTF-8. Access JWT HS256 có issuer/audience/expiry; mỗi request được kiểm lại user/session/version trong DB. Refresh token opaque chỉ lưu hash, cookie HttpOnly/SameSite=Lax/Path=/api/v1/auth, Secure ngoài local. Rotation/replay thu hồi family; logout, đổi mật khẩu, khóa/đổi quyền có cơ chế vô hiệu hóa access cũ.

Các POST register/login/refresh/logout cần Origin trong ALLOWED_ORIGINS, kể cả CLI. Viewer dùng token access trong memory, không localStorage. Single-flight và Web Locks điều phối refresh; BroadcastChannel đồng bộ đổi phiên. Yêu cầu cũ không bị replay sang tài khoản mới khi session thay đổi. Trình duyệt thiếu Web Locks dùng hành vi bảo thủ: nếu refresh race bị phát hiện, cần đăng nhập lại.

Redis limiter dùng Lua atomic counter + expiry: login/register 5 lần/phút/IP, refresh 20 lần/phút/IP và family. Redis lỗi thì auth trả 503 có kiểm soát; catalog vẫn đọc PostgreSQL. Forwarded IP không được tin khi chưa cấu hình proxy tin cậy, vì vậy nhiều client qua cùng proxy local có thể chia sẻ hạn mức; cần chốt trust proxy trước khi mở dịch vụ ra ngoài.

Inventory adjustment cần Idempotency-Key 16–128 ký tự chữ/số/gạch nối/gạch dưới. Khóa được namespace theo actor/operation; cùng payload trả kết quả cũ, khác payload trả409. Stock/history/response/idempotency commit cùng transaction. Admin cuối cùng đang hoạt động không thể bị khóa, xóa hoặc hạ quyền.

## Mua hàng Phase 03

Đăng nhập Customer → mở sản phẩm → thêm giỏ → sửa số lượng → chọn địa chỉ trong tài khoản → xác nhận giá và COD/Mock → đặt hàng. Giỏ chưa giữ hàng; server tính lại quote/stock trong transaction. Khi giá/phí/giỏ thay đổi, UI yêu cầu xem lại và xác nhận lại. Mỗi giỏ tối đa 100 sản phẩm, số lượng mỗi món 1–1000; tiền và tổng tối đa 9×10¹² VND.

Đơn và thanh toán bắt đầu Pending. Admin xử lý COD theo Confirmed → Processing → Shipped → Delivered; chỉ Delivered mới ghi Paid. Mock có thể tạo tối đa 5 attempts; thất bại giữ đơn Pending để thử lại hoặc hủy, thành công ghi Paid và Confirmed nguyên tử. Không hủy đơn Paid/Shipped; không refund ở Base. Admin xem/chuyển trạng thái tại `/#/admin/orders`.

`MOCK_PAYMENTS_ENABLED=true` chỉ dùng development/test. API từ chối cấu hình bật mô phỏng trong production; khi tắt, không đăng ký route tạo attempt/simulate và không nhận checkout Mock. Đây là mô phỏng, không kết nối cổng thanh toán tiền thật.

Checkout/cancel/status/attempt/simulate cần Idempotency-Key. Viewer lưu key và request đang chờ theo tài khoản trong sessionStorage, không lưu token. Nếu mất response, nút kiểm tra lại giữ đúng intent cả sau reload; lỗi xác thực/rate limit không làm mất key. Server lưu kết quả cùng mutation, replay không tạo đơn/thanh toán/hoàn kho trùng. Snapshot sản phẩm/giá/địa chỉ/shipping giữ nguyên sau khi dữ liệu nguồn đổi.

Order/payment/inventory events được ghi vào outbox trong cùng transaction. Chưa có publisher/notification; Phase 05 sẽ xử lý backlog. Sau khi build, có thể kiểm chứng kill/restart API test bằng `python3 scripts/phase03-restart.py` với TEST_DATABASE_URL, TEST_REDIS_URL và E2E_EVIDENCE_DIR; script tạo/dọn schema riêng, dùng port 18081, không dừng API khác.
