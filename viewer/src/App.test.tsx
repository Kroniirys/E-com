import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import App from './pages/FoundationPage'
import { getFoundation } from './client'
vi.mock('./client', () => ({ getFoundation: vi.fn() }))
const data = {
  service: 'ecommerce-api',
  version: 'test',
  environment: 'test' as const,
  database: 'up' as const,
  schema_version: 1,
  store: { name: 'Cửa hàng kiểm thử', currency: 'VND' as const, shipping_fee_minor: 30000 },
}
describe('trạng thái kết nối', () => {
  it('hiển thị dữ liệu do API cung cấp', async () => {
    vi.mocked(getFoundation).mockResolvedValue(data)
    render(<App />)
    expect(await screen.findByText('Cửa hàng kiểm thử')).toBeVisible()
    expect(screen.getByRole('status')).toHaveTextContent('Đã kết nối')
  })
  it('không hiển thị dữ liệu giả khi lỗi và có thể thử lại', async () => {
    vi.mocked(getFoundation)
      .mockRejectedValueOnce(new Error('Database chưa sẵn sàng'))
      .mockResolvedValueOnce(data)
    render(<App />)
    expect(await screen.findByRole('alert')).toHaveTextContent('Database chưa sẵn sàng')
    expect(screen.queryByTestId('store-name')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Kiểm tra lại kết nối' }))
    expect(await screen.findByText('Cửa hàng kiểm thử')).toBeVisible()
  })
})
