'use client'

import SetPasswordForm from '@/components/common/set-password-form'
import { resetPassword } from '@/lib/api/auth'

export default function ResetPasswordPage() {
  return (
    <SetPasswordForm
      title='Đặt lại mật khẩu'
      description='Chọn mật khẩu mới. Mọi phiên đăng nhập cũ sẽ bị đăng xuất.'
      submitLabel='Đặt lại mật khẩu'
      successMessage='Đặt lại mật khẩu thành công, vui lòng đăng nhập'
      submit={resetPassword}
    />
  )
}
