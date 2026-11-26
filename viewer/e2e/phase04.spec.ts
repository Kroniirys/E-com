import { test, expect } from '@playwright/test'
test('Phase04: coupon checkout, Delivered review, moderation và dashboard', async ({ page }) => {
  test.setTimeout(120000)
  const code = 'TEST-' + process.env.APP_VERSION!.slice(-8).toUpperCase()
  const content = 'Đánh giá E2E ' + code
  const login = async (admin: boolean) => {
    await page.goto('/#/login')
    await page
      .getByLabel('Email', { exact: true })
      .fill(admin ? process.env.SEED_ADMIN_EMAIL! : process.env.SEED_CUSTOMER_EMAIL!)
    await page
      .getByLabel('Mật khẩu', { exact: true })
      .fill(admin ? process.env.SEED_ADMIN_PASSWORD! : process.env.SEED_CUSTOMER_PASSWORD!)
    await page.getByRole('button', { name: 'Đăng nhập', exact: true }).click()
    await expect(page.getByRole('button', { name: 'Đăng xuất', exact: true })).toBeVisible()
  }
  const logout = async () => {
    await page.getByRole('button', { name: 'Đăng xuất', exact: true }).click()
    await expect(page.getByRole('link', { name: 'Đăng nhập', exact: true })).toBeVisible()
  }
  await login(true)
  await page.goto('/#/admin/coupons')
  await page.getByRole('button', { name: 'Tạo coupon', exact: true }).click()
  await page.getByLabel('Mã coupon', { exact: true }).fill(code)
  await page.getByLabel('Hết hạn (UTC)', { exact: true }).fill('2027-12-31T23:59')
  await page.getByRole('button', { name: 'Lưu coupon', exact: true }).click()
  await expect(page.getByText(code, { exact: true })).toBeVisible()
  await logout()
  await login(false)
  await page.goto('/#/account')
  await page.getByRole('button', { name: 'Thêm địa chỉ', exact: true }).click()
  await page.getByLabel('Người nhận', { exact: true }).fill('Buyer Phase04')
  await page.getByLabel('Số điện thoại', { exact: true }).fill('0901234567')
  await page.getByLabel('Địa chỉ', { exact: true }).fill('Địa chỉ test Phase04')
  await page.getByRole('button', { name: 'Lưu địa chỉ', exact: true }).click()
  await expect(page.getByText('Địa chỉ test Phase04', { exact: true })).toBeVisible()
  await page.goto('/#/')
  await page.getByRole('heading', { name: 'Đèn bàn Halo', exact: true }).click()
  const productURL = page.url()
  await page.getByRole('button', { name: 'Thêm vào giỏ hàng', exact: true }).click()
  await expect(page.getByText('Đã thêm vào giỏ hàng.', { exact: false })).toBeVisible()
  await page.getByRole('link', { name: 'Giỏ hàng', exact: true }).click()
  await page.getByLabel('Mã giảm giá', { exact: true }).fill(code)
  await page.getByRole('button', { name: 'Áp dụng coupon', exact: true }).click()
  await expect(page.getByText('Đã áp dụng ' + code, { exact: true })).toBeVisible()
  await page
    .getByRole('checkbox', {
      name: 'Tôi xác nhận giá, địa chỉ và phương thức thanh toán hiện tại.',
    })
    .check()
  await page.getByRole('button', { name: 'Đặt hàng', exact: true }).click()
  await expect(page).toHaveURL(/#\/orders\//)
  const orderID = page.url().split('/orders/')[1]
  await expect(page.getByText(new RegExp('Coupon ' + code))).toBeVisible()
  await logout()
  await login(true)
  await page.goto('/#/admin/orders/' + orderID)
  for (const status of ['Confirmed', 'Processing', 'Shipped', 'Delivered']) {
    await page.getByLabel('Trạng thái đơn tiếp theo', { exact: true }).selectOption(status)
    await page.getByRole('button', { name: 'Cập nhật trạng thái đơn', exact: true }).click()
    if (status === 'Delivered') await expect(page.getByText(/^Đã giao ·/)).toBeVisible()
    else
      await expect(
        page
          .getByLabel('Trạng thái đơn tiếp theo', { exact: true })
          .locator(
            `option[value="${status === 'Confirmed' ? 'Processing' : status === 'Processing' ? 'Shipped' : 'Delivered'}"]`,
          ),
      ).toBeAttached()
  }
  await logout()
  await login(false)
  await page.goto(productURL)
  await page.getByLabel('Nội dung đánh giá', { exact: true }).fill(content)
  await page.getByRole('button', { name: 'Gửi đánh giá', exact: true }).click()
  await expect(page.getByText(content, { exact: true }).first()).toBeVisible()
  await logout()
  await login(true)
  await page.goto('/#/admin/reviews')
  const review = page.getByRole('article').filter({ hasText: content })
  await review.getByLabel('Lý do kiểm duyệt', { exact: true }).fill('E2E moderation')
  await review.getByRole('button', { name: 'Ẩn đánh giá', exact: true }).click()
  await expect(review.getByText('Đang ẩn', { exact: false })).toBeVisible()
  await page.goto(productURL)
  await expect(page.getByText(content, { exact: true })).toHaveCount(0)
  await page.goto('/#/admin/dashboard')
  await expect(page.getByRole('heading', { name: 'Tổng quan cửa hàng', exact: true })).toBeVisible()
  await expect(page.getByText('831.000 ₫', { exact: true })).toBeVisible()
  await page.screenshot({ path: process.env.E2E_EVIDENCE_DIR + '/dashboard.png', fullPage: true })
})
