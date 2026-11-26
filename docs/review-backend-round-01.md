# Review toàn bộ backend — Vòng 01

Ngày: 2026-11-26. Phạm vi: toàn bộ mã nguồn hiện tại trong `was/`, migration 1–5, HTTP contract và cấu hình chạy liên quan; bao gồm thay đổi Phase05 chưa commit trên branch `feat/phase-05-cache-events-workers`. Đây là review mã nguồn, không phải phê duyệt artifact hay nghiệm thu Phase06.

## Kết quả

Có **1 finding Medium/P2** được xác nhận. Chưa phát hiện finding Critical/High trong phạm vi đã rà. Finding dưới đây còn mở; vòng này không sửa mã nguồn triển khai.

### BE-R01-01 — Rate limit dùng chung bucket cho các khách qua Nginx

- Mức độ: Medium/P2.
- Vị trí: `viewer/nginx.conf:9`, `was/internal/httpserver/server.go:40`, `was/internal/httpapi/api.go:141`.
- Đường lỗi: Nginx chuyển tiếp `/api/` nhưng không gắn địa chỉ khách; API gọi `SetTrustedProxies(nil)` và dùng `c.ClientIP()` làm khóa rate limit. API vì vậy nhận IP container Nginx cho mọi yêu cầu qua proxy. Chỉ bổ sung header ở Nginx cũng chưa đủ vì API hiện bỏ qua header forwarded.
- Tác động: login 5/phút, register 5/phút và refresh 20/phút trở thành hạn mức dùng chung cho mọi khách đi qua cùng Nginx, thay vì theo IP khách. Khách khác có thể nhận 429 dù chưa đạt hạn mức riêng; 5 lần login thất bại có thể chặn lần login đúng tiếp theo của khách khác trong cửa sổ hiện tại.
- Bằng chứng thực thi: dùng chính Gin đang khóa trong go.mod, tạo hai request httptest có RemoteAddr proxy `172.20.0.5:43000` và X-Forwarded-For lần lượt là `198.51.100.10`, `203.0.113.20`. Với `SetTrustedProxies(nil)`, cả hai trả `ClientIP() = 172.20.0.5`. Cấu hình Nginx hiện còn không tạo header này. Đây là kiểm chứng hàm phân giải IP; vòng này không giả lập hai máy khách thật qua Docker.
- Cách sửa: cấu hình proxy/CIDR tin cậy rõ ràng ở API; để Nginx ghi đè header IP từ địa chỉ kết nối khách (`$remote_addr`). Không bật trust-all và không tin trực tiếp header do client tự gửi. Khi thêm proxy ngoài Nginx cần xác định chuỗi proxy được phép trước khi lấy địa chỉ nguồn.
- Regression cần có: hai IP qua proxy tin cậy dùng bucket độc lập; một IP vẫn bị 429 đúng hạn mức; gọi trực tiếp với header giả không đổi bucket; refresh của các khách khác không bị một khách dùng hết quota chung.

## Phạm vi đã rà

| Nhóm | Nội dung |
| --- | --- |
| Auth, user và HTTP — reviewer độc lập | JWT/session/revocation, refresh rotation/reuse, mật khẩu, RBAC/last admin, ownership địa chỉ, binding, cookie/Origin/CORS, cấu hình và wiring |
| Commerce — reviewer độc lập | Cart/quote, checkout nguyên tử, idempotency, giới hạn tiền/số lượng, stock/payment/cancel, coupon expiry/usage/snapshot, eligibility/moderation review, dashboard và migration 3–4 |
| Catalog, cache và async — reviewer chính | Catalog mutation/hydration, deferred generation và cache snapshot/fallback, migration5, outbox claim/lease/token, mandatory/confirm, inbox/ack/retry/DLQ/replay, notification ownership, worker lifecycle/readiness/metrics và Compose |

Không ghi style hoặc giả thuyết chưa đủ bằng chứng thành finding. Không có finding mới chắc chắn khác trong nhóm commerce/cache/worker qua review này; điều đó không thay thế kiểm thử tải và nghiệm thu tổng hợp.

## Kiểm chứng trong vòng này

- `go test -race ./...`: đạt; các package có thể dùng cache Go. Integration nằm sau build tag nên không được tính là chạy lại bởi lệnh này.
- `go vet ./...`: đạt.
- Reviewer commerce chạy lại unit các package orders/coupons/reviews/admin/money: đạt.
- Chương trình httptest kiểm chứng phân giải IP proxy: hai địa chỉ client khác nhau đều thành IP proxy, xác nhận BE-R01-01.
- Không chạy lại integration PostgreSQL/Redis/RabbitMQ, browser hay Docker fault injection trong vòng review này. Kết quả 39 integration và Docker của Phase05 là bằng chứng từ lượt triển khai trước, không gán thành kết quả mới.
- Không thay đổi mã nguồn triển khai, không commit/push. Chỉ bổ sung báo cáo này.
