# Hạn chế và việc chưa kiểm chứng

## Docker tạm hoãn theo yêu cầu người dùng

Phase 01–03 đã kiểm chứng native trên PostgreSQL, Redis và Chrome thật. Không chạy kiểm tra Docker trong Phase 02/03. Chưa có bằng chứng runtime image/Compose, Nginx/container network hoặc volume restart của cấu trúc hiện tại. Khi tiếp tục kiểm chứng Docker, chạy clean-start trong README và lưu evidence riêng; không dùng kết quả native thay thế.

## Phạm vi còn lại

- Phase 04–06 chưa triển khai: coupon/review/cache catalog, publisher/consumer/notification/dashboard tổng hợp và nghiệm thu toàn hệ thống. API tương lai trả 404.
- Worker chỉ bootstrap DB/schema rồi chờ shutdown; chưa consume message.
- Ảnh lưu metadata/URL HTTP(S), chưa upload hoặc backend-fetch. Trình duyệt cần truy cập được nguồn ảnh.
- Không có payment gateway tiền thật, refund, biến thể, nhiều kho/nhiều người bán hoặc triển khai public trong Base.

## Lưu ý vận hành hiện tại

- API không tin X-Forwarded-For mặc định. Qua Vite/Nginx local, người dùng có thể chung IP rate limit; trước triển khai qua reverse proxy cần cấu hình trust boundary cụ thể và kiểm chứng chống giả IP.
- Web Locks bảo vệ refresh nhiều tab trên trình duyệt hỗ trợ. Nếu thiếu Web Locks và xảy ra refresh replay, server vẫn revoke family; người dùng cần đăng nhập lại.
- Refresh cookie Secure ngoài development/test; cấu hình Origin và HTTPS phải khớp khi triển khai. Access token nằm trong memory, không lưu localStorage.
- `scripts/setup-local.py` tạo secret và demo password local, không ghi đè `.env`. Seed chỉ dành development/test, chạy lại không sửa tài khoản hoặc stock đã có. Không dùng tài khoản demo ở môi trường public.
- Cơ sở dữ liệu schema v3 bắt buộc trước app hiện tại. Rollback schema chỉ trên test có kiểm soát; dữ liệu cần giữ ưu tiên backup/migration sửa tiến.

Xem [tiến độ và bằng chứng](PROGRESS.md), [review Phase 03](review-phase-03.md).

## Giới hạn giao dịch Phase 03

- Mock chỉ là mô phỏng local/test, tối đa5attempts, không cổng thanh toán hoặc tiền thật. Production chỉ COD trong phạm vi đã triển khai; bật mock trong production làm startup thất bại.
- Không có refund; đơn Paid hoặc đã Shipped không được hủy. Nếu điều chỉnh tăng stock tới trần trước khi hoàn đơn, cần giảm stock hợp lệ trước khi hủy; server không bỏ qua trần quantity để hoàn.
- Outbox đã lưu bền nhưng chưa publish; không có notification. Event từ admin adjustment/product/category sẽ được nối đầy đủ ở Phase 05. Log staged có thể thuộc transaction rollback; đối chiếu bảng outbox để biết event đã commit.
- Intent recovery thuộc sessionStorage của tab và tài khoản. Đóng tab/xóa storage có thể mất nút khôi phục; lịch sử đơn vẫn là nguồn đối chiếu, idempotency server không bị xóa theo browser.
