'use client'

import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { ArrowLeft, Mail, Phone } from 'lucide-react'
import { Button } from '@/components/ui/button'

const SUPPORT_EMAIL = process.env.NEXT_PUBLIC_SUPPORT_EMAIL || 'khanhsphg2003@gmail.com'
const SUPPORT_PHONE = process.env.NEXT_PUBLIC_SUPPORT_PHONE || '0354803687'

export default function UniversityContactPage() {
  const router = useRouter()
  return (
    <main className='min-h-screen bg-slate-50 px-4 py-10 dark:bg-background'>
      <section className='mx-auto max-w-2xl rounded-xl bg-white p-6 shadow-sm dark:bg-card md:p-10'>
        <Button variant='ghost' onClick={() => router.push('/')}><ArrowLeft /> Trang chủ</Button>
        <h1 className='mt-6 text-2xl font-semibold text-main'>Dành cho cơ sở đào tạo</h1>
        <p className='mt-4 leading-7 text-muted-foreground'>
          Văn bằng số PQC giúp cơ sở đào tạo quản lý hồ sơ, phát hành văn bằng số ký hậu lượng tử ML-DSA và cung cấp
          cổng xác minh công khai cho nhà tuyển dụng.
        </p>
        <h2 className='mt-6 text-lg font-semibold'>Thông tin trường cần chuẩn bị</h2>
        <ul className='mt-3 list-disc space-y-2 pl-6 text-muted-foreground'>
          <li>Tên chính thức, mã trường và địa chỉ cơ sở đào tạo.</li>
          <li>Website chính thức và tên miền email của trường.</li>
          <li>Email, họ tên và số điện thoại người phụ trách quản trị.</li>
          <li>Thông tin hoặc giấy tờ xác minh tư cách của cơ sở đào tạo.</li>
        </ul>
        <h2 className='mt-6 text-lg font-semibold'>Liên hệ</h2>
        <div className='mt-3 space-y-3'>
          <a className='flex items-center gap-2 text-main underline' href={`mailto:${SUPPORT_EMAIL}`}><Mail /> {SUPPORT_EMAIL}</a>
          <a className='flex items-center gap-2 text-main underline' href={`tel:${SUPPORT_PHONE}`}><Phone /> {SUPPORT_PHONE}</a>
        </div>
        <p className='mt-5 leading-7 text-muted-foreground'>
          Sau khi xác minh thông tin, chúng tôi sẽ tạo tài khoản quản trị và gửi liên kết kích hoạt tới email của
          người phụ trách.
        </p>
        <Link href='/auth/sign-in' className='mt-8 block'><Button className='w-full'>Đăng nhập</Button></Link>
      </section>
    </main>
  )
}
