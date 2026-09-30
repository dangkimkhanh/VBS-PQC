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
import { signIn } from '@/lib/auth/auth'
import { showNotification } from '@/lib/utils/common'
import Link from 'next/link'
import { validateEmail, validatePassword } from '@/lib/utils/validators'
import { useRouter, useSearchParams } from 'next/navigation'
import { Suspense, useEffect } from 'react'
import { LogInIcon, User } from 'lucide-react'
import appLogo from '../../../../public/assets/images/applogo.png'
const formSchma = z.object({
  email: validateEmail,
  password: validatePassword
})

const SignInPage = () => {
  const router = useRouter()
  const searchParams = useSearchParams()
  useEffect(() => {
    if (searchParams.get('expired')) {
      showNotification('warning', 'Phiên đăng nhập đã hết hiệu lực, vui lòng đăng nhập lại')
    }
  }, [searchParams])
  const form = useForm<z.infer<typeof formSchma>>({
    resolver: zodResolver(formSchma),
    defaultValues: {
      email: '',
      password: ''
    }
  })

  const handleSubmit = async (data: z.infer<typeof formSchma>) => {
    const res = await signIn(data)
    if (!res.ok) {
      showNotification('error', res.message || 'Email hoặc mật khẩu không đúng')
    } else {
      showNotification('success', 'Đăng nhập thành công')
      router.refresh()
    }
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
            <span className='text-xl font-semibold md:text-2xl'>Đăng nhập</span>
            <DialogDescription>Dành cho quản trị hệ thống, quản trị trường và sinh viên</DialogDescription>
          </DialogHeader>
          <Form {...form}>
            <form onSubmit={form.handleSubmit(handleSubmit)} className='space-y-4'>
              <CustomFormItem
                type='input'
                control={form.control}
                name='email'
                label='Email'
                placeholder='Email đăng nhập (sinh viên có thể dùng email trường)'
                setting={{ input: { type: 'email' } }}
              />
              <CustomFormItem
                type='input'
                control={form.control}
                name='password'
                label='Mật khẩu'
                placeholder='Nhập mật khẩu'
                setting={{ input: { type: 'password' } }}
              />
              <Button type='submit' className='w-full' isLoading={form.formState.isSubmitting}>
                <LogInIcon /> Đăng nhập
              </Button>
            </form>
          </Form>

          <Link className='text-center text-sm underline' href='/auth/forgot-password'>Quên mật khẩu?</Link>

          <div className='relative'>
            <hr className='my-4' />
            <span className='absolute left-1/2 top-1 -translate-x-1/2 text-sm text-gray-500'>
              <div className='bg-white px-2 text-sm dark:bg-background'>hoặc</div>
            </span>
          </div>
          <p className='text-center text-sm'>Sinh viên chưa có tài khoản?</p>
          <Link href='/auth/sign-up'>
            <Button variant={'outline'} className='w-full'>
              <User /> Kích hoạt tài khoản sinh viên
            </Button>
          </Link>
          <Link className='text-center text-sm text-main underline' href='/auth/university-contact'>
            Trường muốn tham gia hệ thống? Xem hướng dẫn liên hệ
          </Link>
        </DialogContent>
      </Dialog>
    </div>
  )
}

const SignInPageWrapper = () => (
  <Suspense>
    <SignInPage />
  </Suspense>
)

export default SignInPageWrapper
