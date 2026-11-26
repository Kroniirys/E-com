import { expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { Field } from './ui'

it('keeps select labels independent of options and associates help text', () => {
  render(
    <Field label="Trạng thái" hint="Chọn trạng thái sản phẩm.">
      <select defaultValue="Draft">
        <option value="Draft">Bản nháp</option>
        <option value="Active">Đang hoạt động</option>
      </select>
    </Field>,
  )
  const control = screen.getByLabelText('Trạng thái', { exact: true })
  expect(control).toHaveAccessibleName('Trạng thái')
  expect(control).toHaveAccessibleDescription('Chọn trạng thái sản phẩm.')
})
