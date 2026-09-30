'use client'

import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { zodResolver } from '@hookform/resolvers/zod'
import { Form } from '@/components/ui/form'
import CustomFormItem from '@/components/common/ct-form-item'
import { Button } from '@/components/ui/button'
import Image from 'next/image'
import background from '../../../../public/assets/images/background.jpg'
import { showNotification } from '@/lib/utils/common'
import { validateEmail, validateNewPassword } from '@/lib/utils/validators'
import Link from 'next/link'
import { InputOTPGroup, InputOTPSeparator } from '@/components/ui/input-otp'
import { InputOTPSlot } from '@/components/ui/input-otp'
import { InputOTP } from '@/components/ui/input-otp'
import { useState } from 'react'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import useSWRMutation from 'swr/mutation'
import { registerAccount, sendOTP, verifyOTP } from '@/lib/api/auth'
import appLogo from '../../../../public/assets/images/applogo.png'
import { REGEXP_ONLY_DIGITS } from 'input-otp'
import { signIn } from '@/lib/auth/auth'
import { useRouter } from 'next/navigation'
import { ArrowLeft, Check, Send, UserPlus } from 'lucide-react'
const formPersonalEmailSchma = z
  .object({
    email: validateEmail,
    password: validateNewPassword,
    confirmPassword: z.string()
  })
  .refine((data) => data.password === data.confirmPassword, {
    message: 'Mật khẩu nhập lại không khớp',
    path: ['confirmPassword']
  })

const AuthPage = () => {
  const [isOTPSended, setIsOTPSended] = useState(false)
  const [isOTPVerified, setIsOTPVerified] = useState(false)
  const formPersonalEmail = useForm<z.infer<typeof formPersonalEmailSchma>>({
    resolver: zodResolver(formPersonalEmailSchma),
    defaultValues: {
      email: '',
      password: '',
      confirmPassword: ''
    }
  })
  const router = useRouter()
  const [inputEmail, setInputEmail] = useState('')
  const [inputOTP, setInputOTP] = useState('')
  const [idUserAfterVerifyOTP, setIdUserAfterVerifyOTP] = useState('')
  const [otpVerificationToken, setOtpVerificationToken] = useState('')
  const mutateSendOTP = useSWRMutation('/auth/request-otp', () => sendOTP(inputEmail.trim()), {
    onSuccess: (data) => {
      showNotification('success', data?.message || `Mã OTP đã được gửi đến email ${inputEmail}`)
      setIsOTPSended(true)
    },
    onError: (error) => {
      showNotification('error', error.message || error.error)
    }
  })

  const mutateVerifyOTP = useSWRMutation('/auth/verify-otp', () => verifyOTP(inputEmail.trim(), inputOTP), {
      onSuccess: (data) => {
      showNotification('success', 'Xác thực thành công')
      setIsOTPVerified(true)
        setIdUserAfterVerifyOTP(data.user_id)
        setOtpVerificationToken(data.verification_token)
    },
    onError: (error) => {
      showNotification('error', error.message || error.error || 'Xác thực thất bại')
    }
  })

  const mutateRegisterAccount = useSWRMutation(
    '/auth/register',
    (_, { arg }: { arg: any }) => registerAccount(arg.email, arg.password, idUserAfterVerifyOTP, otpVerificationToken),
    {
      onSuccess: async () => {
        showNotification('success', 'Kích hoạt tài khoản thành công')
        const res = await signIn({
          email: formPersonalEmail.getValues('email'),
          password: formPersonalEmail.getValues('password')
        })
        router.push(res.ok ? '/student' : '/auth/sign-in')
        router.refresh()
      },
      onError: (error) => {
        showNotification('error', error.message || error.error || 'Đăng ký tài khoản thất bại')
      }
    }
  )

  const handleSubmit = async (data: z.infer<typeof formPersonalEmailSchma>) => {
    mutateRegisterAccount.trigger({
      email: data.email,
      password: data.password
    })
  }

  return (
    <div className='relative bottom-0 left-0 right-0 top-0 h-screen'>
      <Image src={background} width={1500} height={1500} className='h-full w-full object-cover' alt='no-image' />
      <Dialog open>
        <DialogContent className='rounded-lg sm:max-w-[450px] [&>button]:hidden'>
          <DialogHeader>
            <DialogTitle>
              <span
                className='flex cursor-pointer items-center justify-center gap-2'
                onClick={() => {
                  router.push('/')
                }}
              >
                <Image src={appLogo} alt='Logo hệ thống văn bằng số PQC' width={50} height={50} />
                <span className='text-xl font-semibold text-main md:text-2xl'>Văn bằng số PQC</span>
              </span>
            </DialogTitle>
            <span className='text-xl font-semibold md:text-2xl'>Kích hoạt tài khoản sinh viên</span>
            <DialogDescription>
              Dành cho sinh viên đã có hồ sơ do trường tạo. Tài khoản chỉ dùng để xem thông tin và văn bằng của bạn.
            </DialogDescription>
          </DialogHeader>

          <div className={`${isOTPSended ? 'hidden' : 'block'}`}>
            <div>
              <Label>Email sinh viên do trường cấp</Label>
              <Input
                className='mt-2'
                type='email'
                placeholder='VD: nguoidung@donvi.edu.vn'
                value={inputEmail}
                onChange={(e) => setInputEmail(e.target.value)}
              />
              <p className='mt-2 text-sm text-gray-500'>
                Mã OTP gồm 6 chữ số sẽ được gửi tới email này nếu bạn có hồ sơ chưa kích hoạt.
              </p>
              <Button
                className='mt-4 w-full'
                onClick={() => mutateSendOTP.trigger()}
                disabled={!/^\S+@\S+\.\S+$/.test(inputEmail.trim())}
                isLoading={mutateSendOTP.isMutating}
              >
                <Send /> Gửi mã OTP
              </Button>
            </div>
          </div>
          <div className={`${isOTPSended ? 'block' : 'hidden'} ${isOTPVerified ? 'hidden' : 'block'}`}>
            <Label>Mã OTP</Label>
            <div className='mb-2' />
            <InputOTP maxLength={6} value={inputOTP} onChange={(e) => setInputOTP(e)} pattern={REGEXP_ONLY_DIGITS}>
              <InputOTPGroup>
                <InputOTPSlot index={0} />
                <InputOTPSlot index={1} />
                <InputOTPSlot index={2} />
              </InputOTPGroup>
              <InputOTPSeparator />
              <InputOTPGroup>
                <InputOTPSlot index={3} />
                <InputOTPSlot index={4} />
                <InputOTPSlot index={5} />
              </InputOTPGroup>
            </InputOTP>
            <p className='mt-2 text-sm text-gray-500'>
              Mã OTP được gửi đến email <b>{inputEmail}</b>, có hiệu lực 5 phút. Sai quá 5 lần cần yêu cầu mã mới.
            </p>
            <div className='my-4 flex w-full gap-4'>
              <Button
                variant={'outline'}
                className='flex-1'
                onClick={() => {
                  setIsOTPSended(false)
                  setIsOTPVerified(false)
                }}
                type='button'
              >
                <ArrowLeft /> Quay lại
              </Button>
              <Button
                className='flex-1'
                variant={'secondary'}
                onClick={() => mutateSendOTP.trigger()}
                isLoading={mutateSendOTP.isMutating}
              >
                <Send /> Gửi lại mã OTP
              </Button>
            </div>
            <Button
              className='w-full'
              onClick={() => mutateVerifyOTP.trigger()}
              disabled={inputOTP.length !== 6}
              isLoading={mutateVerifyOTP.isMutating}
            >
              <Check /> Xác thực
            </Button>
          </div>

          {/* Form email cá nhân */}
          <Form {...formPersonalEmail}>
            <form
              onSubmit={formPersonalEmail.handleSubmit(handleSubmit)}
              className={`${isOTPVerified && isOTPSended ? 'block' : 'hidden'} space-y-4`}
            >
              <CustomFormItem
                type='input'
                control={formPersonalEmail.control}
                name='email'
                label='Email cá nhân'
                description='Dùng để đăng nhập và nhận thông báo văn bằng sau khi ra trường'
                placeholder='VD: abc@gmail.com'
                setting={{ input: { type: 'email' } }}
              />
              <CustomFormItem
                type='input'
                control={formPersonalEmail.control}
                name='password'
                label='Mật khẩu'
                placeholder='Tối thiểu 8 ký tự'
                setting={{ input: { type: 'password' } }}
              />
              <CustomFormItem
                type='input'
                control={formPersonalEmail.control}
                name='confirmPassword'
                label='Nhập lại mật khẩu'
                placeholder='Nhập lại mật khẩu'
                setting={{ input: { type: 'password' } }}
              />
              <Button type='submit' className='w-full' isLoading={formPersonalEmail.formState.isSubmitting}>
                <UserPlus /> Kích hoạt tài khoản
              </Button>
            </form>
          </Form>

          <div className='text-center text-sm'>
            Đã có tài khoản?{' '}
            <Link className='underline underline-offset-4' href='/auth/sign-in'>
              Đăng nhập
            </Link>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}

export default AuthPage
