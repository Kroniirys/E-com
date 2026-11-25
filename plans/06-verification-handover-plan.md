---
title: "Phase 06 — Kiểm chứng và bàn giao"
artifact: implementation-plan
role: fullstack
status: draft
version: 0.2
owner: "TBD"
approvers: []
last_updated: 2026-11-25
---

# Phase 06 — Kiểm chứng và bàn giao

## 1. Mục tiêu và đầu vào

Chứng minh Base đầy đủ, có thể tái lập và bàn giao từ repository chuẩn. Đầu vào: [plan tổng](00-master-plan.md), Phase 01–05 đạt tiêu chí với evidence; [Phase 05](05-cache-events-workers-plan.md) đã hoàn thiện toàn stack. Phase này tổng hợp và bổ sung khoảng trống kiểm chứng, không bắt đầu viết toàn bộ test từ đầu.

## 2. Phạm vi

Hồi quy nghiệp vụ, concurrency/failure recovery, security, load test bằng k6, clean-start, báo cáo và hướng dẫn bàn giao. Không tự thêm microservices, Kubernetes, thanh toán thật, CI/CD hoặc tính năng ngoài Base để vượt nghiệm thu.

## 3. Công việc

### P06-01 — Ma trận nghiệm thu và môi trường kiểm chứng

- Phạm vi file: `docs/acceptance-matrix.md`, fixtures và scripts test.
- Map từng nhóm yêu cầu §5 plan tổng → công việc `Pxx-nn` → test/API/UI → evidence → trạng thái Đạt/Không đạt/Chưa kiểm chứng.
- Ghi phiên bản mã nguồn, dependency, cấu hình, dữ liệu fixture và môi trường chạy. Fixture riêng cho admin/customer/đơn ở các trạng thái và coupon biên; không dùng dữ liệu cá nhân thật.
- Kiểm tra: không nhóm nào thiếu coverage, không “Đạt” khi chỉ đọc code; việc môi trường chưa hỗ trợ phải ghi nguyên nhân và điều kiện chạy lại.
- Phụ thuộc: Phase 01–05.

### P06-02 — Hồi quy từ API/UI tới dữ liệu

- Phạm vi file: `was/tests/`, frontend test/E2E, scripts test và `docs/testing-report.md`.
- Build Go/frontend; chạy unit/integration/E2E đã có, `go test ./...`, `go test -race ./...` khi môi trường hỗ trợ, static check và frontend lint/typecheck theo script thực tế.
- E2E: đăng ký/login/profile/address; admin user/product/category/inventory; tìm hàng/cart/checkout COD và mock success/failure/retry; cancel; coupon; Delivered/review/moderation; notification/dashboard.
- Negative: thiếu quyền/ownership, input sai, product ẩn, giá/stock/coupon thay đổi, order transition sai, token hết hạn/replay và duplicate request.
- Đối chiếu SQL cho tồn kho, tổng tiền, usage, payment và outbox/notification thay vì chỉ kiểm status HTTP.
- Kiểm tra: test pass/fail/skipped có log, fixture cleanup cô lập và report không che skipped thành pass.
- Phụ thuộc: P06-01.

### P06-03 — Concurrency và khả năng phục hồi

- Chạy lại test nhiều buyer tranh stock, cùng user checkout/cập nhật cart, adjustment đua checkout, coupon lượt cuối, callback đua cancel và hoàn kho/payment trùng.
- Kill/restart API/worker, ngắt Redis/RabbitMQ, DB lỗi giữa transaction; sau phục hồi đối chiếu invariant và backlog tiêu thụ.
- Thử duplicate/out-of-order, retry/DLQ/replay và cache invalidate race; API không tạo order vượt stock và worker không nhân notification.
- Kiểm tra: stock không âm, không order/paid/hoàn kho trùng, transaction lỗi không có side effect nửa chừng; lỗi hạ tầng có evidence trước/sau phục hồi.
- Phụ thuộc: P06-02.

### P06-04 — Kiểm tra bảo mật và cấu hình

- Kiểm tra password hashing, JWT validation, refresh rotation/revocation, cookie/Origin-CSRF/CORS, rate limit, RBAC/IDOR, mass assignment, input validation, SQL injection và XSS trong review/product content.
- Quét secret trong file được bàn giao và lịch sử Git nếu có commit; rà env/seed/demo credentials, log, dependency vulnerability và quyền truy cập metrics/broker management.
- Error response không lộ stack trace/config/DB detail; security headers và giới hạn request được kiểm tra. Mock payment route không hoạt động khi tắt chế độ demo.
- Upload không thuộc Base theo thiết kế; nếu đã bổ sung phải kiểm tra đầy đủ loại/kích thước/quyền/lưu trữ theo plan bổ sung.
- Đầu ra: kết quả, evidence và cách xử lý trong testing report/known issues. Lỗi có thể vượt quyền, lộ secret hoặc làm sai dữ liệu là blocker bàn giao.
- Phụ thuộc: P06-01/02.

### P06-05 — Load test và benchmark thực tế

- Phạm vi file: `was/tests/load/` dùng k6 và `docs/benchmark-report.md`.
- Ghi CPU/RAM/môi trường/container limits, dataset, cache warm/cold và cấu hình chạy. Dùng user/cart/key riêng, không để test hết stock rồi hiểu nhầm là lỗi hạ tầng.
- Kịch bản: catalog search/list/detail, login có kiểm soát rate limit, checkout và mixed traffic; tách xung đột nghiệp vụ dự kiến với lỗi server.
- Chạy mức 50 và 100 virtual users; 300 chỉ khi môi trường cho phép. Đây là mức tải kiểm chứng, không phải cam kết năng lực hoặc ước lượng công việc.
- Thu RPS, average/P95/P99 latency, error rate, CPU/memory, DB pool và queue/outbox backlog; mô tả cách tái lập và nguyên nhân giới hạn.
- Không tự đặt SLO để báo pass. Nếu cần gate hiệu năng, chốt threshold trước bài đo chính thức theo môi trường; nếu chưa có, bàn giao số liệu và hạn chế trung thực.
- Kiểm tra: script/report/raw result nhất quán, không ghi số chưa chạy, dọn fixture và xác nhận invariant sau tải.
- Phụ thuộc: P06-02–04, metrics Phase 05.

### P06-06 — Clean-start, migration và phục hồi dữ liệu

- Dùng DB/volume test mới, thực hiện `docker compose up --build` theo README, chạy migration/seed và đi lại luồng demo customer/admin.
- Thử nâng cấp từ schema phase trước theo migration liên tiếp; seed lặp không tạo trùng hoặc ghi đè order thật. Kiểm tra shutdown/restart giữ dữ liệu.
- Kiểm chứng backup/restore PostgreSQL ở môi trường cô lập, kiểm tra startup khi migration sai và hướng dẫn sửa tiến/rollback app tương thích.
- Script reset phải có guard và chỉ rõ dữ liệu sẽ xóa; không tự chạy lên dữ liệu cần giữ.
- Kiểm tra: người khác theo đúng README chạy được, không cần AI hoặc bước thủ công ẩn.
- Phụ thuộc: P06-03/04.

### P06-07 — Sửa lỗi trong phạm vi và bàn giao

- Sửa lỗi phát hiện trong các kiểm tra trên, thêm regression test cho lỗi có rủi ro lặp; chạy lại phần bị ảnh hưởng và bộ critical flow khi cần.
- Repository là nguồn chuẩn: mã nguồn, Compose/Dockerfile, migration/seed/scripts/test, `.env.example`, OpenAPI, architecture/ERD/decision log, events/operations, testing/benchmark reports, README và known issues.
- Không copy source sang một cây `deliverables/source-code` để duy trì thêm bản khác. Nếu cần bản nén bàn giao, sinh từ revision đã kiểm chứng và ghi revision/checksum.
- README có lệnh chạy/test/migrate/seed/reset, endpoint docs/health, cách cấp tài khoản demo từ cấu hình, cách demo COD/mock và các giới hạn đã biết.
- Báo cáo số module/endpoint/test đã kiểm chứng dựa trên kết quả thật; liệt kê fail/skipped/flow chưa đạt và hướng khắc phục.
- Phụ thuộc: P06-01–06.

## 4. Quản lý kết quả và lỗi chưa xử lý

Bất kỳ lỗi làm sai tiền/stock, vượt quyền, lộ secret, mất sự kiện đã commit hoặc thiếu chức năng trong phạm vi Base đều chưa đạt nghiệm thu cho tới khi xử lý và có bằng chứng. Known issues không thay thế việc hoàn thành yêu cầu bắt buộc. Hạn chế ngoài phạm vi phải nêu rõ và không được quảng bá thành tính năng đã có. Nếu thiếu môi trường kiểm chứng thì giữ trạng thái Chưa kiểm chứng, không kết luận Base hoàn thành.

## 5. Tiêu chí hoàn thành

- [ ] Ma trận phủ toàn bộ phạm vi nguồn, mỗi nhóm có evidence phù hợp.
- [ ] Build/unit/integration/E2E/concurrency/recovery và security đạt; không còn blocker bắt buộc.
- [ ] Benchmark có dữ liệu thật, môi trường/script/raw result để chạy lại.
- [ ] Clean-start, migration/seed, restart và backup/restore đã kiểm chứng.
- [ ] Docs/API/ERD/reports khớp revision source bàn giao; không secret hoặc số liệu giả.
- [ ] Customer/admin có thể thực hiện demo Base mà không cần AI hỗ trợ khởi chạy.

## 6. Checklist review

- [ ] Không lấy thiếu môi trường hoặc test skipped làm lý do báo pass.
- [ ] Report đối chiếu được với command log, dữ liệu và revision.
- [ ] Không bổ sung tính năng ngoài Base để né việc sửa lỗi hiện có.
- [ ] Bản bàn giao chỉ có một nguồn mã chuẩn và hướng dẫn chạy đủ rõ.

Đối chiếu hoàn thành toàn bộ với [plan tổng](00-master-plan.md).
