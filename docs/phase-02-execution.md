# Checklist thực thi Phase 02

Phạm vi được người dùng giao: triển khai Phase 02, sau đó tạo develop và merge các branch tính năng vào develop. Docker tạm hoãn. Không áp gate artifact của dự án TeemStone vào E-com.

| ID | File/module thực thi | Thứ tự / đầu ra | Kiểm chứng |
| --- | --- | --- | --- |
| P02-01 | was/migrations/000002_*, modules/users models, modules/catalog, seed | Schema trước service; seed lấy tài khoản từ env | Fresh/upgrade migration, lặp seed, FK/unique |
| P02-02 | modules/auth, platform/limit, config/security, cmd/api | Sau schema; JWT/session/refresh/Redis | Replay/race/revoke/Origin/rate limit down |
| P02-03 | modules/users service, httpapi users | Sau auth; admin/profile/address | IDOR/mass assignment/last-admin race |
| P02-04 | modules/catalog, httpapi catalog | Sau auth/schema; products/categories/images | Atomic update, public visibility, filter/sort |
| P02-05 | modules/inventory, httpapi inventory | Sau catalog; adjustment/idempotency/history | Stock race, double-submit, rollback, overflow |
| P02-06 | viewer/src modules/session/catalog/profile/admin | Sau contract/services; UI thật theo quyền | Component + browser E2E customer/admin |
| P02-07 | was/tests/integration/phase02_test.go, viewer/e2e, docs | Kiểm chứng theo từng bước và tổng hợp | Native PostgreSQL/Redis/Chrome, không Docker |

Catalog gộp product/category trong một module vì transaction assignment cần phối hợp chặt; service và HTTP handler vẫn tách ranh giới. Schema dùng auth_sessions riêng để thu hồi access token theo family và giữ refresh history chống replay. Role/status thay đổi serialize bằng row khóa role Admin rồi user; refresh/password/logout khóa user trước session/token. Address thay default khóa user trước addresses. Adjustment reference là UUID operation, không raw key để tránh xung đột giữa hai admin.

Refresh replay thu hồi family và commit trước khi trả 401; client dùng Web Locks cho cross-tab refresh và single-flight trong tab. Các phiên trình duyệt không hỗ trợ Web Locks dùng hành vi bảo thủ: replay vẫn revoke, user cần đăng nhập lại. Không nới lỏng replay security chỉ để che race.

## Kết quả

P02-01–07 hoàn thành trên native PostgreSQL/Redis/Chrome. Evidence và số bài kiểm tra được ghi trong [progress](PROGRESS.md); findings cùng cách sửa trong [review](review-phase-02.md). Helper `was/cmd/testscope` tạo/dọn schema E2E có ownership guard. Script `scripts/setup-local.py` sinh `.env` mode 0600, không in secret và không ghi đè cấu hình cũ. Viewer có script format/format:check để giữ TSX dễ review.

Git bàn giao theo yêu cầu: branch tính năng Phase 02 và branch tích hợp `develop`; giữ lịch sử Phase 01, không force push. Kết quả SHA và push được báo trong chat sau khi Git xác nhận.
