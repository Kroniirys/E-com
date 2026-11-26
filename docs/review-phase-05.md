# Review Phase 05

Ngày2026-11-26. Sub-agent phase01_reviewer review chỉ đọc nhiều vòng: generation/cache race/visibility, AMQP channel/confirm/return/deadline, SQL claim/inbox/ownership, retry/DLQ/replay, metrics, CLI cleanup và UI. Agent chính sửa và kiểm chứng; không có thao tác lên dữ liệu hoặc queue ngoài namespace test.

| Mức độ | Finding thật | Sửa / bằng chứng |
| --- | --- | --- |
| Medium | Custom TCP Dial chỉ có connect timeout, AMQP handshake với peer im lặng có thể treo | Socket deadline3s trước handshake, thư viện clear sau khi mở; fake TCP regression. Publish gate có context, deadline tổng3s cho chờ gate/write/confirm, không vượt lease do chờ mutex. |
| Low | payment.completed không có status nên body notification trống trạng thái | Set Paid và ánh xạ trạng thái tiếng Việt; AMQP integration assert body Đã thanh toán. |

Một nghi vấn metrics404 đã được rút sau khi truy vết API.Register; endpoint được mount đúng từ đầu, không ghi thành defect đã sửa. Docker build đầu phát hiện snapshot Viewer còn lỗi kiểu unread_count; đã sửa, build cuối và browser Docker đạt. Test metrics được sửa để đo counter trước restart, vì counter process reset là hành vi đúng.

Đã đóng các gap bằng AMQP thật: hai relay8event→8publication; publish rồi mất mark vẫn cùng ID; commit inbox/notification rồi đóng channel chưa ack→Redelivered→không trùng; transient retry đủ3lần→DLQ attempt4→restoreDB→replay thành công; poison version vàoDLQ ngay. Không coi database processor test là thay thế broker test.

Vòng cuối không còn finding mã nguồn/contract cần sửa trong phạm vi được rà. Đã đối chiếu39integration PASS,9Dockerchecks PASS,5nativeE2E và1DockerNginx E2E PASS. Ảnh mobile hiển thị đúng unread/read và không tràn. [Raw evidence](PROGRESS.md), [vận hành](operations.md).

Không hứa exactly-once ngoài DB hoặc email thật. Benchmark/backup-restore/security audit tổng hợp thuộc Phase06; Docker đã kiểm local development, chưa đồng nghĩa deploy public được nghiệm thu.
