# Review Phase 03

Ngày: 2026-11-26. Phạm vi P03-01–06: schema, cart/quote, money, idempotency, checkout/lifecycle/payment/outbox, HTTP contract/RBAC, Viewer intent recovery và bằng chứng thực tế. Sub-agent `phase01_reviewer` review độc lập chỉ đọc nhiều vòng; agent chính sửa và kiểm chứng. Không Docker.

## Findings đã xử lý

| Mức độ | Vấn đề | Sửa và regression |
| --- | --- | --- |
| High | GORM Take dùng lại cart DTO có UUID vừa sinh sau INSERT ON CONFLICT; cart đã tồn tại bị lọc sai ID | Reset Row{} trước SELECT theo user_id. Test cart lần đầu đồng thời, add/get/update/checkout và E2E đều qua DB thật. |
| High | Admin transition và customer cancel dùng chung operation/fingerprint; sau demotion có thể replay cached đơn người khác qua route customer | Tách orders.admin-transition và orders.transition; privilege guard trước replay. HTTP regression admin cancel → demote → login Customer → cùng key: customer404, admin403, không lộ snapshot. |
| Medium | Client xóa intent khi gặp mọi4xx, kể cả429 từ refresh; có thể mất key của request trước đã commit nhưng mất response | Chỉ giải phóng key khi success hoặc business rejection xác định; giữ lỗi auth/rate/session/network/5xx. Unit lost-response →429→retry giữ key; E2E mất response/reload phục hồi đúng order. |
| Low | Record idempotency lưu200 trong khi checkout/attempt trả201 | Lưu201 cho hai operation tạo tài nguyên; replay handler và record khớp contract. |

Trong E2E cũng sửa logout không ghi đè điều hướng mới, chờ trạng thái logout hoàn tất trước login tiếp; badge catalog cũ đã được thay bằng hướng dẫn mua hàng. Suite E2E trước dùng chung limiter gây login429 đúng policy; runner nay cô lập API/schema/prefix theo spec, không nới hạn mức.

## Kết luận

Vòng review cuối không còn finding mã nguồn/contract cần sửa được phát hiện. Đã rà khóa/transaction/failure path, stock và money bounds, owner/role replay, snapshot, attempt partial unique, late callback, mock disable/production policy, UI intent resumption và auth epoch. Đã xem ảnh order mobile, admin Delivered và mock payment; không có clipping/tràn đáng kể.

Bằng chứng: 13 Go unit dưới race detector, vet, 24 integration, 13 Viewer unit/component và 3 Chrome E2E đạt. Kill/restart API vẫn trả đúng response đã commit và một order/một lần trừ stock. [Progress và raw evidence](PROGRESS.md).

Log outbox ở mức staged chỉ ghi event/aggregate/request ID, không payload/address; record outbox đã commit là nguồn chuẩn. Không coi log staged là bằng chứng đã publish. Review không thay phê duyệt artifact và không bao phủ Docker hoặc publisher/consumer Phase05.
