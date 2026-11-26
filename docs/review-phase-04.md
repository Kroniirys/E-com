# Review Phase 04

Ngày2026-11-26. Sub-agent phase01_reviewer review độc lập nhiều vòng: coupon money/hash/locking/usage, review eligibility/ownership/visibility, dashboard/date/precision, seed/migration và UI. Có một lần lỗi credits, sau khi tiếp tục đã chạy lại và hoàn tất review.

Finding Medium: response moderation khai báo Review với additionalProperties=false nhưng mã trả AdminReview có metadata. Đã đổi ref AdminReview và thêm HTTP regression kiểm moderation_reason/moderated_by/moderated_at. Mô tả quyền của eligibility/my reviews đã đồng bộ Customer. Tự rà bổ sung: form edit coupon chuyển expiry về UTC trước đưa vào datetime-local.

Reviewer đối chiếu31integration PASS,4E2E PASS,13Viewer unit và Go race/vet/build. Legacy record trước migration3→4 replay giữ hash/result, rollback guard giữ audit, dashboard loại state không hợp lệ và tổng vượt trần một order. Seed opt-in có Delivered thật và chạy lặp không trùng. Không còn finding mã nguồn/contract cần sửa được phát hiện trong phạm vi đã rà.

Account bổ sung danh sách review riêng để owner có thể xóa review bị ẩn hoặc sản phẩm không còn public; endpoint luôn lọc owner. Không có publisher/notification/cache catalog giả. Docker chưa kiểm tra theo yêu cầu. Xem [progress](PROGRESS.md).
