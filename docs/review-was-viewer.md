# Review cấu trúc WAS/Viewer

Reviewer độc lập: sub-agent `phase01_reviewer`. Phạm vi: ownership thư mục, Go imports/embed, root/WAS Makefiles, Viewer package/lock/generator, integration/E2E runner, đường dẫn cấu hình Docker và liên kết tài liệu. Không chạy kiểm tra Docker theo yêu cầu người dùng.

## Finding đã sửa

Medium: E2E runner và screenshot từng ghi cố định vào evidence Phase 01, có thể làm mất bằng chứng lịch sử. Đã thêm `E2E_EVIDENCE_DIR`, chuẩn hóa đường dẫn tại runner và truyền cùng giá trị cho API log/ảnh Playwright. Lượt kiểm chứng mới dùng thư mục restructure riêng.

## Kết quả

Không còn finding đường dẫn hoặc wiring cần sửa được phát hiện trong phạm vi review. Go module/contract nằm trong WAS, Viewer build đọc đúng contract mới, Make/script nối hai phần đúng. PostgreSQL integration 2 PASS và Chrome E2E 1 PASS; build/lint/typecheck/unit/contract đạt. Reviewer đối chiếu độc lập 19 file evidence lịch sử giữ nguyên SHA-256.

Xem [progress và logs](PROGRESS.md). Docker tạm hoãn, không ghi nhận cấu hình hoặc runtime Docker ở cấu trúc mới đã được kiểm chứng.
