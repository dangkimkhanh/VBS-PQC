'use client'

import SetPasswordForm from '@/components/common/set-password-form'
import { activateAccount } from '@/lib/api/auth'

export default function ActivateAccountPage() {
  return (
    <SetPasswordForm
      title='Kích hoạt tài khoản quản trị trường'
      description='Đặt mật khẩu cho tài khoản. Liên kết kích hoạt chỉ dùng được một lần.'
      submitLabel='Đặt mật khẩu và kích hoạt'
      successMessage='Kích hoạt tài khoản thành công, vui lòng đăng nhập'
      submit={activateAccount}
    />
  )
}
