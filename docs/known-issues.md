# Hạn chế và việc chưa kiểm chứng

## Docker được tạm hoãn theo yêu cầu người dùng

Yêu cầu mới nhất cho phép tạm thời chưa kiểm tra Docker; phần này không chặn công việc tách WAS/Viewer. Không chạy lại Docker/Compose trong lượt cơ cấu lại.

Mã nguồn Phase 01 đã build và kiểm chứng native trên PostgreSQL/Chrome thật; cấu hình Compose hợp lệ. Chưa kiểm chứng chạy thực tế image/stack bằng Compose vì không có quyền Docker daemon. Rootless cũng thiếu uidmap/subuid/subgid; không có quyền cài/sửa cấu hình hệ thống trong phiên này.

Ảnh hưởng: chưa đủ bằng chứng nghiệm thu tiêu chí API/viewer/PostgreSQL chạy qua Compose, chưa xác nhận runtime Nginx/container network/volume restart. Native E2E và manifest image hợp lệ không thay thế kiểm tra này. Xem [progress](PROGRESS.md) và README để chạy lại khi có môi trường.

## Giới hạn phạm vi chủ đích

- Chỉ Phase 01 đã có mã nguồn. Auth/catalog/cart/order/payment/coupon/review/cache/events/notification là thiết kế cho phase sau; endpoint đó chưa đăng ký và trả 404.
- Worker hiện bootstrap DB/schema/seed và shutdown, chưa consume message.
- Chưa có tài khoản demo nghiệp vụ trước Phase 02; seed hiện là cấu hình cửa hàng thật.
- Không upload file, payment gateway, refund, biến thể, nhiều kho, nhiều người bán hoặc deployment public trong Base.
- Local credentials trong .env.example là mẫu, không là secret dùng cho môi trường khác. Seed chỉ cho development/test, không ghi đè dữ liệu đã có.
- UI trạng thái không tự polling; dùng nút kiểm tra lại khi dependency phục hồi. Swagger local dùng đường dẫn file rõ ràng để chạy được cả Vite và Nginx.
