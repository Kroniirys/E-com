# Tiến độ triển khai

Cập nhật 2026-11-26. Không có ước lượng thời gian.

## Phase 01 — Mã nguồn và kiểm chứng native đã hoàn tất; Docker tạm hoãn

Đã triển khai các công việc trong [plan Phase 01](../plans/01-foundation-and-design-plan.md), có sub-agent `phase01_reviewer` review độc lập và sửa lặp. Không còn finding mã nguồn/contract cần sửa được phát hiện trong vòng review cuối. Theo yêu cầu mới nhất, tạm hoãn kiểm chứng Docker; không ghi Docker pass và không dùng phần hoãn này để chặn việc cơ cấu lại WAS/Viewer.

| Công việc | Kết quả | Đầu ra / evidence |
| --- | --- | --- |
| P01-01 | Đạt | [Requirements](requirements.md), [decision log](architecture-decisions.md) |
| P01-02 | Đạt | [Architecture](architecture.md), [ERD 25 bảng thiết kế](erd.md), [state machine](order-payment-state-machine.md), [events](events.md) |
| P01-03 | Đạt | [API design](api-design.md), [OpenAPI](../was/api/openapi.yaml): 61 operations/41 schemas, 4 endpoint hiện thực |
| P01-04 | Mã nguồn đạt; Compose runtime chưa kiểm chứng | Go API/worker/config/logging/shutdown, React UI thật, Dockerfile/Compose; native smoke/E2E đạt |
| P01-05 | Đạt trên PostgreSQL thật | Migration up/repeat/rollback/re-up, seed lặp/không ghi đè, schema test riêng, reset guard |
| P01-06 | Đạt tài liệu và native clean path; cần bổ sung kết quả Compose khi có quyền | README, progress, known issues, review và logs |

## Kết quả kiểm chứng đã chạy

Môi trường: Go 1.27.4, Node 24.22.3, npm 11.19.1, PostgreSQL 17.12 chạy độc lập trên localhost, Google Chrome 157 qua Playwright 1.65.1. Dependency được khóa trong `was/go.mod`, `was/go.sum` và `viewer/package-lock.json`. Binary cuối build với CGO_ENABLED=0 giống Dockerfile.

| Kiểm tra | Lệnh / phương pháp | Kết quả | Evidence |
| --- | --- | --- | --- |
| Go build | `make GO=/snap/go/current/bin/go build` | Đạt: api/worker/migrate/seed/healthcheck static | [log](evidence/phase-01/build-go.txt) |
| Go unit/race | `go test -race -p 2 ./...` với cache trong /tmp | Đạt: 7 test | [log](evidence/phase-01/go-race.txt) |
| Go vet | `go vet ./...` | Đạt, exit 0 | [log](evidence/phase-01/go-vet.txt) |
| PostgreSQL integration | `go test -tags=integration -count=1 -v ./tests/integration` với TEST_DATABASE_URL *_test | Đạt: 2 test, schema cô lập được dọn | [log](evidence/phase-01/integration.txt) |
| OpenAPI | `npm run validate:api`, generation TypeScript | Đạt, không warning schema | [log](evidence/phase-01/openapi.txt) |
| Reproducible dependency install | `npm ci` | Đạt từ lockfile | [log](evidence/phase-01/npm-ci.txt) |
| Frontend lint/typecheck | `npm run lint`, `npm run typecheck` | Đạt | [lint](evidence/phase-01/frontend-lint.txt), [types](evidence/phase-01/frontend-typecheck.txt) |
| Frontend unit | `npm test` | Đạt: 2 test dữ liệu thật từ client, lỗi và retry | [log](evidence/phase-01/frontend-tests.txt) |
| Frontend build | `npm run build` | Đạt | [log](evidence/phase-01/frontend-build.txt) |
| Chrome E2E | `make test-e2e` với TEST_DATABASE_URL và Chrome sẵn có | Đạt: 1 test API thật → UI, reload, mobile không tràn, Swagger local; xác minh version riêng để không chạy nhầm backend | [log](evidence/phase-01/e2e.txt), [desktop](evidence/phase-01/desktop.png), [mobile](evidence/phase-01/mobile.png) |
| Runtime smoke | Config fail-fast; stop/restart PostgreSQL thật; API/worker SIGTERM | Đạt: 9 checks | [JSON](evidence/phase-01/native-smoke.json), [API log](evidence/phase-01/smoke-api.log), [worker log](evidence/phase-01/smoke-worker.log) |
| Image tags | `docker manifest inspect` cho 5 image dùng trong đường chạy chính | Tồn tại trên registry, không thay bằng chứng runtime | [JSON](evidence/phase-01/image-manifests.json) |
| Compose syntax/wiring | `docker compose --env-file .env.example config --quiet`, mặc định và đủ profiles | Đạt cấu hình, không phải bằng chứng runtime | [log](evidence/phase-01/compose-validation.txt) |
| Compose runtime/clean-start | `docker compose up --build` | Chưa kiểm chứng | Không có quyền daemon; rootless thiếu system requirements |

Unit/component tests được phân biệt với integration thật. Không test integration/E2E nào bị skip rồi coi pass. Native smoke dùng PostgreSQL tạm do phiên triển khai tạo trong /tmp, không dừng database có sẵn của người dùng. Log vet rỗng là output thành công, không phải chưa chạy.

## Review và sửa lặp

Xem [review Phase 01](review-phase-01.md). Đã sửa guard query override DB, admin create/update contract, cookie header contract, E2E chạy nhầm server, Swagger directory routing và ignore metadata build. Vòng cuối không có finding mã nguồn/contract cần sửa.

## Docker — ghi nhận lịch sử và tạm hoãn hiện tại

Docker CLI/Compose có sẵn nhưng kết nối `/var/run/docker.sock` bị permission denied; `sudo -n` yêu cầu mật khẩu. Kiểm tra rootless cho thấy thiếu newuidmap/newgidmap và subordinate UID/GID. Không sửa quyền socket, user hoặc cấu hình hệ thống để vượt giới hạn này.

Khi môi trường có Docker daemon phù hợp, thực hiện đường clean-start trong README trên volume local mới có kiểm soát, xác nhận migration/seed/API/frontend và restart giữ dữ liệu, lưu logs rồi mới đánh dấu tiêu chí Compose hoàn thành. Đây là trạng thái lịch sử tại Phase 01; kết quả Phase 02 được bổ sung bên dưới. Kiểm chứng Docker hiện được tạm hoãn theo yêu cầu của chủ dự án.

## Cập nhật cấu trúc WAS/Viewer

Đã thực hiện yêu cầu tách mã nguồn và review lại bằng sub-agent độc lập. Không còn finding đường dẫn/wiring cần sửa được phát hiện. Evidence Phase 01 trước khi tách giữ nguyên, kiểm bằng SHA-256 của 19 file; kết quả mới lưu riêng.

- Backend: `was/`, gồm Go module, api contract/embed, cmd/internal/migrations/tests/scripts/bin, Makefile và Dockerfile.
- Frontend: `viewer/`, gồm npm project, src/E2E/public/config, Dockerfile/Nginx; generator đọc `was/api/openapi.yaml`.
- Root: docs/plans, environment/Compose/Make điều phối và scripts/run-e2e.py; không còn Go module hoặc source backend tại root.
- E2E runner dùng `was/bin/` và `viewer/`; `E2E_EVIDENCE_DIR` được chuẩn hóa và truyền cho log/ảnh để không ghi đè evidence lịch sử.

| Kiểm tra sau khi tách | Kết quả | Evidence |
| --- | --- | --- |
| `make was-check build` với Go binary hiện có | Đạt 7 unit tests, vet và 5 binary static | [log](evidence/restructure-was-viewer/was-check-build.txt) |
| `make viewer-check` | Đạt OpenAPI/generation/lint/typecheck, 2 unit tests và build | [log](evidence/restructure-was-viewer/viewer-check.txt) |
| `make test-integration` với PostgreSQL test native | Đạt 2 integration tests từ root Makefile qua WAS | [log](evidence/restructure-was-viewer/integration.txt) |
| `make test-e2e` với Chrome/PostgreSQL thật | Đạt 1 E2E: WAS → Viewer → Swagger | [log](evidence/restructure-was-viewer/e2e.txt), [desktop](evidence/restructure-was-viewer/desktop.png), [mobile](evidence/restructure-was-viewer/mobile.png) |
| Cấu trúc, link, lock metadata và evidence lịch sử | Đạt, 19 file lịch sử giữ nguyên hash | [kết quả](evidence/restructure-was-viewer/layout-verification.txt) |
| Docker/Compose | Không chạy theo yêu cầu tạm hoãn | Chỉ sửa đường dẫn trong cấu hình; không tuyên bố đã kiểm chứng Docker ở cấu trúc mới |

Chi tiết review: [WAS/Viewer review](review-was-viewer.md). Các log phía trên trong thư mục phase-01 là bằng chứng lịch sử trước khi tách, không bị viết lại thành log của cấu trúc mới.

## Phase 02 — Hoàn tất triển khai và kiểm chứng native (2026-11-26)

Đã thực hiện P02-01–07 trong [plan](../plans/02-identity-catalog-inventory-plan.md), giữ ranh giới `was/` và `viewer/`. Review độc lập đã kết thúc, không còn finding cần sửa trong phạm vi đã rà. Docker tiếp tục tạm hoãn; Phase 03 chưa bắt đầu.

| Công việc | Kết quả / đầu ra |
| --- | --- |
| P02-01 | Schema v2 gồm 13 bảng ứng dụng; seed admin/customer/catalog từ env, lặp không ghi đè dữ liệu |
| P02-02 | bcrypt/JWT/session, refresh rotation/replay, revoke, RBAC, Origin và Redis rate limit thật |
| P02-03 | Admin users CRUD/search/status, profile/address/default/ownership và bảo vệ admin cuối cùng |
| P02-04 | Product/category/images CRUD, public visibility, search/filter/sort/pagination; dữ liệu thật |
| P02-05 | Stock adjustment/history nguyên tử, row lock, idempotency theo actor/operation/key và chống âm |
| P02-06 | Storefront/detail, đăng ký/login/account và admin UI; token memory, refresh nhiều tab và epoch guard |
| P02-07 | OpenAPI/ERD/README/decision log cập nhật, kiểm chứng và review có evidence |

| Kiểm tra | Kết quả | Evidence |
| --- | --- | --- |
| Go unit dưới race detector | Đạt 10 test | [log](evidence/phase-02/go-race.txt) |
| Go vet | Đạt, exit 0; log rỗng | [log](evidence/phase-02/go-vet.txt) |
| PostgreSQL + Redis integration | Đạt 12 test; JWT/revoke/replay, RBAC/IDOR, admin race, catalog rollback, stock concurrency/idempotency, Redis fail-closed, migration/seed và 36 route/contract | [log](evidence/phase-02/integration.txt) |
| Viewer và contract | Đạt OpenAPI/generation, lint/typecheck, 9 unit/component test, build, định dạng | [log](evidence/phase-02/viewer-check.txt) |
| Build Go và Chrome E2E | Đạt 5 binary static và 2 E2E: nền tảng; admin catalog/stock và customer/account/session/password | [log](evidence/phase-02/e2e.txt) |
| UI desktop/mobile | Không tràn ngang; catalog/admin hiển thị dữ liệu từ cùng backend/DB test | [admin](evidence/phase-02/admin-products.png), [storefront](evidence/phase-02/storefront.png), [mobile](evidence/phase-02/storefront-mobile.png) |
| Docker | Tạm hoãn theo yêu cầu người dùng | Không chạy và không báo pass |

Môi trường native bổ sung Redis 7.4.12. Integration và E2E dùng database tên *_test, schema riêng và Redis key prefix riêng; dọn đúng tài nguyên được tạo. Không FLUSH Redis hoặc sửa dữ liệu development. Test không bị skip. Log [E2E ban đầu](evidence/phase-02/e2e-initial-failure.txt) được giữ để truy vết lỗi label select đã sửa; [lượt cuối](evidence/phase-02/e2e.txt) đạt. Evidence Phase 01 và sau cơ cấu thư mục được giữ nguyên.

Chi tiết: [review Phase 02](review-phase-02.md), [checklist thực thi](phase-02-execution.md), [hạn chế còn lại](known-issues.md). Checkout/payment và chống overselling trong checkout sẽ được kiểm chứng riêng ở Phase 03; kết quả adjustment hiện tại không thay thế nghiệm thu đó.

## Phase 03 — Hoàn tất luồng mua hàng và kiểm chứng native (2026-11-26)

Đã thực hiện P03-01–06 trong [plan](../plans/03-cart-orders-payments-plan.md). Review độc lập nhiều vòng, sửa lỗi và có regression; không còn finding cần sửa được phát hiện ở vòng cuối. Docker tiếp tục tạm hoãn. Phase 04 chưa bắt đầu.

| Công việc | Kết quả / đầu ra |
| --- | --- |
| P03-01 | Schema v3, 8 bảng bổ sung (21 bảng ứng dụng tổng); nâng cấp từ v2 và rollback schema test riêng |
| P03-02 | Cart PostgreSQL/version, live quote/availability, tiền nguyên có giới hạn; không giữ stock |
| P03-03 | Checkout snapshot và stock/history/payment/outbox/idempotency cùng commit, khóa theo thứ tự và retry lỗi DB có giới hạn |
| P03-04 | Lifecycle/history/ownership, COD Paid khi Delivered, mock failure/retry/Paid, cancel/Failed hoàn một lần |
| P03-05 | UI cart/checkout/customer/admin orders, xác nhận lại quote và giữ intent khi mất response/reload |
| P03-06 | Kiểm thử thật, restart, OpenAPI/ERD/state machine/README và review/evidence |

| Kiểm chứng | Kết quả | Evidence |
| --- | --- | --- |
| Go unit/race | Đạt 13 test; gồm tiền/state machine/mock configuration | [log](evidence/phase-03/go-race.txt) |
| Go vet | Đạt, exit0, log rỗng | [log](evidence/phase-03/go-vet.txt) |
| Build Go | Đạt 5 binary static; E2E còn build helper testscope | [build](evidence/phase-03/build-go.txt), [E2E](evidence/phase-03/e2e.txt) |
| PostgreSQL/Redis integration | Đạt24 test: 12 nền tảng/Phase02 và12 Phase03 | [log](evidence/phase-03/integration.txt) |
| Viewer/contract | Đạt13 unit/component, OpenAPI/generation/lint/typecheck/build/format | [log](evidence/phase-03/viewer-check.txt) |
| Chrome E2E | Đạt3 spec: nền tảng, Phase02 và COD/mock/admin/lost-response Phase03; mỗi spec có schema/API/rate prefix riêng | [log](evidence/phase-03/e2e.txt) |
| API kill/restart | Đạt: response replay giống hệt, một order, một lần trừ stock, cart rỗng | [JSON](evidence/phase-03/restart.json), [log](evidence/phase-03/restart.txt) |
| UI desktop/mobile | Hiển thị đúng COD/Delivered, mock failed→retry→Paid và mobile không tràn | [mobile](evidence/phase-03/order-mobile.png), [admin](evidence/phase-03/admin-order-delivered.png), [mock](evidence/phase-03/mock-payment.png) |
| Docker | Không chạy theo yêu cầu tạm hoãn | Không báo pass |

SQL đối chiếu trong integration: 20 khách tranh stock5 → đúng5order, stock0, 5checkout histories, 5order.created và5inventory.updated. Mười hai success requests đồng thời → một attempt Paid và một payment.completed. Cùng cart/key, key khác cùng snapshot, cart mutation, admin adjustment, payment/cancel và Delivered replay không sinh side effect thừa. Lỗi ghi outbox được inject sau các mutation để chứng minh rollback cả transaction. Có regression HTTP admin cancel đơn người khác rồi bị hạ quyền: cùng key qua customer route không được nhận cached snapshot.

E2E Phase03 bỏ response sau route.fetch đã tạo đơn, reload cart rỗng rồi khôi phục đúng order bằng key cũ; tiếp tục admin Delivered COD, mock Failure→retry→Paid và hủy COD. Log [E2E lỗi đầu](evidence/phase-03/e2e-initial-failure.txt) giữ để truy vết điều hướng logout đã sửa; fixture suite sau đó được cô lập để giữ rate policy5/phút thay vì nới hạn mức. [Lượt cuối](evidence/phase-03/e2e.txt) đạt. Evidence Phase01/02 giữ nguyên.

Outbox chỉ ghi bền và chưa publish; không có notification. Mock chỉ local/test, không payment gateway. Chi tiết [review](review-phase-03.md), [checklist](phase-03-execution.md), [hạn chế](known-issues.md).

Tổng hợp kiểm chứng Phase03: [verification.json](evidence/phase-03/verification.json). Sau các lượt chạy, [kiểm tra cleanup](evidence/phase-03/cleanup.txt) xác nhận không còn schema test riêng. Metadata OpenAPI cuối (0.3.0 và nhãn Phase03 đã hiện thực) được [validate/generate/kiểm tra embed](evidence/phase-03/contract-final.txt) lại.

## Phase 04 — Coupon, review và dashboard (2026-11-26)

Hoàn thành P04-01–06 theo [plan](../plans/04-coupons-reviews-admin-plan.md), schema4 với24bảng ứng dụng. Coupon fixed/percentage, preview không giữ lượt; checkout kiểm lại terms/expiry/limit và commit usage/snapshot/stock/order/outbox cùng transaction. Hủy không hoàn lượt. Review chỉ từ Delivered, unique cả soft-delete, public count/average cùng visibility snapshot. Dashboard [from,to) theo IANA timezone, ngày tạo đơn, tổng Delivered+Paid decimal string chính xác.

| Kiểm chứng | Kết quả | Evidence |
| --- | --- | --- |
| Go unit/race và vet | Đạt16unit; vet exit0 | [race](evidence/phase-04/go-race.txt), [vet](evidence/phase-04/go-vet.txt) |
| Go static build | Đạt5binary | [log](evidence/phase-04/build-go.txt) |
| PostgreSQL/Redis integration | Đạt31test | [log](evidence/phase-04/integration.txt) |
| Viewer/contract | Đạt13unit, OpenAPI/generation/lint/typecheck/build/format | [log](evidence/phase-04/viewer-check.txt) |
| Chrome E2E | Đạt4spec gồm hồi quy Phase01–03 và coupon→checkout→Delivered→review→moderation→dashboard | [log](evidence/phase-04/e2e.txt), [dashboard](evidence/phase-04/dashboard.png) |
| Docker | Tạm hoãn theo yêu cầu | Không chạy, không báo pass |

Integration bổ sung:20buyer tranh3lượt coupon, chỉ3order/usage; cancel/replay không thay used_count; fault outbox rollback usage/stock; review đồng thời chỉ1bản, xóa mềm không cho tạo lại; hidden review không vào public aggregate; dashboard loại Pending/Cancelled/Delivered-unpaid và tổng9000000031000 vượt trần một order. Persist record/hash4-field ở schema3 trước nâng cấp rồi replay bằng schema4 không đổi ID/tiền. Rollback4 từ chối xóa review/audit. Seed opt-in tạo Delivered/review qua service và chạy2lần không nhân bản.

Phase03 đã push/merge develop tại12ec68d, gồm9f6cec9 (code) và7b44028 (docs). Phase04 ở branch local feat/phase-04-coupons-reviews-admin, chưa commit/push. [Review độc lập](review-phase-04.md); evidence các phase cũ giữ nguyên. Phase05 chưa bắt đầu.
