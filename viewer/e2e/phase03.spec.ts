import { test, expect } from '@playwright/test'

test('Phase 03: COD, mất response/reload/retry, admin giao hàng và mock thất bại rồi thành công', async ({
  page,
}) => {
  test.setTimeout(120000)
  const output = process.env.E2E_EVIDENCE_DIR!
  const login = async (email: string, password: string) => {
    await page.goto('/#/login')
    await page.getByLabel('Email', { exact: true }).fill(email)
    await page.getByLabel('Mật khẩu', { exact: true }).fill(password)
    await page.getByRole('button', { name: 'Đăng nhập', exact: true }).click()
    await expect(page.getByRole('button', { name: 'Đăng xuất', exact: true })).toBeVisible()
  }
  await login(process.env.SEED_CUSTOMER_EMAIL!, process.env.SEED_CUSTOMER_PASSWORD!)
  await page.getByRole('link', { name: 'Tài khoản', exact: true }).click()
  await page.getByRole('button', { name: 'Thêm địa chỉ', exact: true }).click()
  await page.getByLabel('Người nhận', { exact: true }).fill('Khách mua Phase 03')
  await page.getByLabel('Số điện thoại', { exact: true }).fill('0901234567')
  await page.getByLabel('Địa chỉ', { exact: true }).fill('Địa chỉ giao hàng kiểm thử Phase 03')
  await page.getByRole('button', { name: 'Lưu địa chỉ', exact: true }).click()
  await expect(page.getByText('Địa chỉ giao hàng kiểm thử Phase 03', { exact: true })).toBeVisible()
  const add = async () => {
    await page.getByRole('link', { name: 'Cửa hàng', exact: true }).click()
    await page.getByRole('heading', { name: 'Đèn bàn Halo', exact: true }).click()
    await page.getByRole('button', { name: 'Thêm vào giỏ hàng', exact: true }).click()
    await expect(page.getByText('Đã thêm vào giỏ hàng.', { exact: false })).toBeVisible()
    await page.getByRole('link', { name: 'Giỏ hàng', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Giỏ hàng', exact: true })).toBeVisible()
  }
  await add()
  let loseResponse = true
  let committedOrder = ''
  await page.route('**/api/v1/orders', async (route) => {
    if (route.request().method() === 'POST' && loseResponse) {
      loseResponse = false
      const response = await route.fetch()
      expect(response.status()).toBe(201)
      committedOrder = (await response.json()).data.id
      await route.abort('failed')
    } else await route.continue()
  })
  await page
    .getByRole('checkbox', {
      name: 'Tôi xác nhận giá, địa chỉ và phương thức thanh toán hiện tại.',
    })
    .check()
  await page.getByRole('button', { name: 'Đặt hàng', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Kiểm tra yêu cầu đặt hàng trước' })).toBeVisible()
  await page.reload()
  await expect(page.getByText('Giỏ hàng đang trống.', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Kiểm tra yêu cầu đặt hàng trước' }).click()
  await expect(page).toHaveURL(new RegExp(`#/orders/${committedOrder}$`))
  await expect(page.getByText('Chưa thanh toán', { exact: true })).toBeVisible()
  await page.setViewportSize({ width: 375, height: 812 })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy()
  await page.screenshot({ path: `${output}/order-mobile.png`, fullPage: true })
  await page.setViewportSize({ width: 1440, height: 1000 })
  await page.getByRole('button', { name: 'Đăng xuất', exact: true }).click()
  await expect(page.getByRole('link', { name: 'Đăng nhập', exact: true })).toBeVisible()
  await login(process.env.SEED_ADMIN_EMAIL!, process.env.SEED_ADMIN_PASSWORD!)
  await page
    .getByRole('navigation', { name: 'Quản trị cửa hàng' })
    .getByRole('link', { name: 'Đơn hàng', exact: true })
    .click()
  await page
    .getByRole('link')
    .filter({ hasText: `Đơn ${committedOrder}` })
    .click()
  for (const status of ['Confirmed', 'Processing', 'Shipped', 'Delivered']) {
    await page.getByLabel('Trạng thái đơn tiếp theo', { exact: true }).selectOption(status)
    await page.getByLabel('Lý do cập nhật', { exact: true }).fill(`E2E ${status}`)
    await page.getByRole('button', { name: 'Cập nhật trạng thái đơn', exact: true }).click()
    if (status === 'Delivered') await expect(page.getByText(/^Đã giao ·/)).toBeVisible()
    else {
      const next =
        status === 'Confirmed' ? 'Processing' : status === 'Processing' ? 'Shipped' : 'Delivered'
      await expect(
        page
          .getByLabel('Trạng thái đơn tiếp theo', { exact: true })
          .locator(`option[value="${next}"]`),
      ).toBeAttached()
    }
  }
  await expect(page.getByText('Đã thanh toán', { exact: true })).toBeVisible()
  await page.screenshot({ path: `${output}/admin-order-delivered.png`, fullPage: true })
  await page.getByRole('button', { name: 'Đăng xuất', exact: true }).click()
  await expect(page.getByRole('link', { name: 'Đăng nhập', exact: true })).toBeVisible()
  await login(process.env.SEED_CUSTOMER_EMAIL!, process.env.SEED_CUSTOMER_PASSWORD!)
  await add()
  await page.getByLabel('Phương thức thanh toán', { exact: true }).selectOption('Mock')
  await page
    .getByRole('checkbox', {
      name: 'Tôi xác nhận giá, địa chỉ và phương thức thanh toán hiện tại.',
    })
    .check()
  await page.getByRole('button', { name: 'Đặt hàng', exact: true }).click()
  await page.getByRole('button', { name: 'Tạo lần thanh toán mô phỏng', exact: true }).click()
  await page.getByRole('button', { name: 'Mô phỏng thất bại', exact: true }).click()
  await expect(page.getByRole('list', { name: 'Các lần thanh toán' })).toContainText('Thất bại')
  await expect(page.getByText('Chưa thanh toán', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Tạo lần thanh toán mô phỏng', exact: true }).click()
  await page.getByRole('button', { name: 'Mô phỏng thành công', exact: true }).click()
  await expect(page.getByText(/^Đã xác nhận ·/)).toBeVisible()
  await expect(page.getByText('Đã thanh toán', { exact: true }).first()).toBeVisible()
  await expect(page.getByRole('button', { name: 'Hủy đơn hàng', exact: true })).toHaveCount(0)
  await page.screenshot({ path: `${output}/mock-payment.png`, fullPage: true })
  await add()
  await page
    .getByRole('checkbox', {
      name: 'Tôi xác nhận giá, địa chỉ và phương thức thanh toán hiện tại.',
    })
    .check()
  await page.getByRole('button', { name: 'Đặt hàng', exact: true }).click()
  page.once('dialog', (dialog) => dialog.accept())
  await page.getByRole('button', { name: 'Hủy đơn hàng', exact: true }).click()
  await expect(page.getByText(/^Đã hủy ·/)).toBeVisible()
  await page.getByRole('link', { name: 'Đơn hàng', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Đơn hàng của tôi', exact: true })).toBeVisible()
})
