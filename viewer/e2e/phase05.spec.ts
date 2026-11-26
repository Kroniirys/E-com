import { test, expect } from '@playwright/test'
test('Phase05: order event → worker → notification UI và đánh dấu đã đọc', async ({ page }) => {
  test.setTimeout(60000)
  const id = crypto.randomUUID().slice(0, 8)
  await page.goto('/#/register')
  await page.getByLabel('Họ và tên', { exact: true }).fill('Buyer Phase05')
  await page.getByLabel('Email', { exact: true }).fill(`worker-${id}@e2e.test`)
  await page.getByLabel(/^Mật khẩu/).fill('Phase05-browser-password-123')
  await page.getByRole('button', { name: 'Tạo tài khoản', exact: true }).click()
  await expect(page.getByRole('link', { name: 'Tài khoản', exact: true })).toBeVisible()
  await page.getByRole('link', { name: 'Tài khoản', exact: true }).click()
  await page.getByRole('button', { name: 'Thêm địa chỉ', exact: true }).click()
  await page.getByLabel('Người nhận', { exact: true }).fill('Buyer Phase05')
  await page.getByLabel('Số điện thoại', { exact: true }).fill('0901234567')
  await page.getByLabel('Địa chỉ', { exact: true }).fill('Địa chỉ kiểm thử worker')
  await page.getByRole('button', { name: 'Lưu địa chỉ', exact: true }).click()
  await expect(page.getByText('Địa chỉ kiểm thử worker', { exact: true })).toBeVisible()
  await page.getByRole('link', { name: 'Cửa hàng', exact: true }).click()
  await page.getByRole('heading', { name: 'Đèn bàn Halo', exact: true }).click()
  await page.getByRole('button', { name: 'Thêm vào giỏ hàng', exact: true }).click()
  await expect(page.getByText('Đã thêm vào giỏ hàng.', { exact: false })).toBeVisible()
  await page.getByRole('link', { name: 'Giỏ hàng', exact: true }).click()
  await page
    .getByRole('checkbox', {
      name: 'Tôi xác nhận giá, địa chỉ và phương thức thanh toán hiện tại.',
    })
    .check()
  await page.getByRole('button', { name: 'Đặt hàng', exact: true }).click()
  await expect(page).toHaveURL(/#\/orders\//)
  const orderID = page.url().split('/orders/')[1]
  await page.getByRole('link', { name: 'Thông báo', exact: true }).click()
  await expect(async () => {
    await page.getByRole('button', { name: 'Làm mới thông báo', exact: true }).click()
    await expect(
      page.getByRole('heading', { name: 'Đặt hàng thành công', exact: true }),
    ).toBeVisible()
  }).toPass({ timeout: 15000, intervals: [500] })
  const notification = page.getByRole('article').filter({ hasText: orderID })
  await expect(notification).toHaveCount(1)
  await notification.getByRole('button', { name: 'Đánh dấu đã đọc', exact: true }).click()
  await expect(page.getByText('0 thông báo chưa đọc', { exact: true })).toBeVisible()
  await page.reload()
  await expect(page.getByText('0 thông báo chưa đọc', { exact: true })).toBeVisible()
  await page.setViewportSize({ width: 375, height: 812 })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy()
  await page.screenshot({
    path: process.env.E2E_EVIDENCE_DIR + '/notifications-mobile.png',
    fullPage: true,
  })
})
