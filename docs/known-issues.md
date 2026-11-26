# Hạn chế và việc chưa kiểm chứng

## Docker đã kiểm chứng trong Phase 05

Yêu cầu tạm hoãn trước đây đã được thay bằng yêu cầu kiểm Docker cùng Phase05. Đã build/start thật trên project ecom-phase05-verify với volume/cổng riêng, kiểm Nginx/API/worker/Redis/RabbitMQ/Prometheus và failure/recovery/restart. Các log cũ giữ nguyên là lịch sử, không sửa thành kết quả Docker pass của thời điểm trước.

## Phạm vi còn lại

- Phase06 chưa hoàn tất: nghiệm thu tổng hợp, security audit cuối, benchmark k6 và backup/restore/handover.
- Worker đã relay/consume/retry/DLQ; mock email chỉ console, không gửi email thật.
- Ảnh lưu metadata/URL HTTP(S), chưa upload hoặc backend-fetch. Trình duyệt cần truy cập được nguồn ảnh.
- Không có payment gateway tiền thật, refund, biến thể, nhiều kho/nhiều người bán hoặc triển khai public trong Base.

## Lưu ý vận hành hiện tại

- API không tin X-Forwarded-For mặc định. Qua Vite/Nginx local, người dùng có thể chung IP rate limit; trước triển khai qua reverse proxy cần cấu hình trust boundary cụ thể và kiểm chứng chống giả IP.
- Web Locks bảo vệ refresh nhiều tab trên trình duyệt hỗ trợ. Nếu thiếu Web Locks và xảy ra refresh replay, server vẫn revoke family; người dùng cần đăng nhập lại.
- Refresh cookie Secure ngoài development/test; cấu hình Origin và HTTPS phải khớp khi triển khai. Access token nằm trong memory, không lưu localStorage.
- `scripts/setup-local.py` tạo secret và demo password local, không ghi đè `.env`. Seed chỉ dành development/test, chạy lại không sửa tài khoản hoặc stock đã có. Không dùng tài khoản demo ở môi trường public.
- Cơ sở dữ liệu schema v5 bắt buộc trước app hiện tại. Rollback schema chỉ trên test có kiểm soát; dữ liệu cần giữ ưu tiên backup/migration sửa tiến.

Xem [tiến độ và bằng chứng](PROGRESS.md), [review Phase 03](review-phase-03.md).

## Giới hạn giao dịch Phase 03

- Mock chỉ là mô phỏng local/test, tối đa5attempts, không cổng thanh toán hoặc tiền thật. Production chỉ COD trong phạm vi đã triển khai; bật mock trong production làm startup thất bại.
- Không có refund; đơn Paid hoặc đã Shipped không được hủy. Nếu điều chỉnh tăng stock tới trần trước khi hoàn đơn, cần giảm stock hợp lệ trước khi hủy; server không bỏ qua trần quantity để hoàn.
- Outbox đã publish/consume trong Phase05, gồm product/category/admin adjustment. Log staged có thể thuộc transaction rollback; đối chiếu outbox/inbox thay vì coi log là bằng chứng commit.
- Intent recovery thuộc sessionStorage của tab và tài khoản. Đóng tab/xóa storage có thể mất nút khôi phục; lịch sử đơn vẫn là nguồn đối chiếu, idempotency server không bị xóa theo browser.

## Lưu ý Phase 05

Cache vẫn đọc DB generation để bảo đảm visibility; chưa benchmark khả năng chịu tải. Khi restore DB/đổi dataset cần đổi namespace hoặc dọn cache có kiểm soát. Worker dùng at-least-once, notification DB chống trùng; console mock email có thể lặp, không hứa exactly-once ngoài DB. Retry transient tối đa3lần rồi DLQ; cần inspect/replay khi dependency phục hồi sau khi đã hết retry.

Docker hiện kiểm ở local development với HTTP/AMQP nội bộ. Chưa là chứng nhận triển khai public, trust reverse proxy/HTTPS và bảo mật môi trường thực vẫn cần kiểm theo Phase06. Không publish management/metrics ra Internet.
