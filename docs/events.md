# Thiết kế sự kiện và background job

Đây là contract thiết kế cho Phase 03–05. Phase 01 chưa có publisher/consumer. RabbitMQ không thay transaction PostgreSQL và không quyết định tính đúng của tiền/stock/coupon.

## Envelope version 1

| Trường | Kiểu / ý nghĩa |
| --- | --- |
| event_id | UUID server sinh, giữ nguyên khi retry/replay |
| type | Tên event trong bảng bên dưới |
| version | Số nguyên 1, version schema payload |
| aggregate_type / aggregate_id | Loại entity và UUID nguồn |
| aggregate_version | Version tăng đơn điệu của entity, dùng kiểm tra event cũ |
| occurred_at | RFC3339 UTC tại transaction phát sinh |
| correlation_id | Request ID hoặc job correlation ID, không chứa token/secret |
| payload | Object có schema theo type, không chứa địa chỉ đầy đủ/password/token |

Không đổi nghĩa field trong cùng version. Consumer không hiểu version phải đưa vào đường lỗi có thể quan sát, không ack bỏ qua im lặng. Broker không public cho client tự publish sự kiện nghiệp vụ.

## Event và payload tối thiểu

| Type | Aggregate / payload | Consumer / mục đích |
| --- | --- | --- |
| order.created | order; order_id, user_id, status, currency, total_minor | Notification tạo đơn |
| order.confirmed | order; order_id, user_id, status | Notification xác nhận |
| order.cancelled | order; order_id, user_id, status, reason_code | Notification hủy |
| order.status_changed | order; order_id, user_id, from_status, to_status | Notification Processing/Shipped/Delivered/Failed |
| payment.completed | payment; payment_id, order_id, user_id, method, amount_minor | Notification thanh toán |
| inventory.updated | inventory; product_id, quantity, inventory_version | Invalidate catalog, không trừ stock lần nữa |
| product.changed | product; product_id, product_version, change_kind | Invalidate detail/list/visibility |
| category.changed | category; category_id, category_version, change_kind | Invalidate category và list liên quan |
| notification.requested | notification job; source_event_id, user_id, kind, reference_id | Chỉ dùng nếu tách job nội bộ, không tạo nguồn notification thứ hai |

Mỗi lifecycle transition có một mapping notification duy nhất. Order Confirmed dùng order.confirmed, Cancelled dùng order.cancelled; order.status_changed dành các transition còn lại, tránh hai event cho cùng thông báo. Inventory/payment event là outcome đã commit, consumer không thực hiện lại mutation nguồn.

## Giao dịch và độ tin cậy

1. Producer ghi mutation + history + outbox cùng transaction. Rollback không để event tồn tại riêng.
2. Relay claim batch bằng lock/lease, publish durable/persistent với confirms và mandatory return; chỉ đánh published sau broker xác nhận route hợp lệ.
3. Có thể crash sau publish trước mark; phát lại cùng event ID. Không hứa exactly-once delivery.
4. Consumer ghi processed_events `(consumer,event_id)` và side effect DB cùng transaction, ack sau commit. Restart trước ack không tạo thêm notification.
5. Retry hữu hạn có backoff và metadata; lỗi schema/permanent sang DLQ. Chỉ ack bản gốc khi đường retry/DLQ đã nhận an toàn. Công cụ replay giữ ID để dedupe.
6. Event cũ không overwrite state mới. Notification history giữ occurred_at, không lấy thứ tự xử lý làm thứ tự nghiệp vụ. Mock email/log có thể lặp sau crash, không hứa exactly-once ngoài DB.

## Topology đề xuất và observability

Exchange topic durable `ecommerce.events`; queue `notifications.v1` nhận order/payment events; queue `catalog-invalidation.v1` nhận inventory/product/category changes. Retry/DLQ tách theo consumer, không dùng requeue nóng vô hạn. Chốt concurrency/prefetch/retry/retention bằng cấu hình và test trong Phase 05.

Log theo event/correlation/aggregate ID. Metric outbox pending/age, confirm failure, retry/DLQ và consumer outcome, không dùng user/order/event ID làm metric label. Broker down chỉ giữ backlog outbox, không rollback order đã commit. Không purge outbox/queue để che lỗi.
