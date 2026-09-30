import Header from '@/components/common/header'
import appLogo from '../../public/assets/images/applogo.png'

import Image from 'next/image'
import { getSession } from '@/lib/auth/session'
import SearchVerifyCode from '@/components/common/search-verify-code'
import Footer from '@/components/common/footer'

const HomePage = async () => {
  const session = await getSession()

  return (
    <main className='flex min-h-screen flex-col'>
      <Header role={session?.role ? (session.role as 'admin' | 'student' | 'university_admin') : null} />
      <section className='container mt-16 flex flex-1 flex-col items-center py-10'>
        <Image src={appLogo} alt='Văn bằng số PQC' width={56} height={56} />
        <h1 className='mt-4 text-center text-2xl font-semibold sm:text-4xl'>Xác minh văn bằng số</h1>
        <p className='mb-8 mt-3 max-w-2xl text-center text-muted-foreground'>
          Văn bằng được ký số hậu lượng tử ML-DSA và ghi nhận trên Blockchain. Quét mã QR trên văn bằng hoặc nhập mã
          xác minh để kiểm tra tính xác thực.
        </p>
        <SearchVerifyCode />
      </section>
      <Footer />
    </main>
  )
}

export default HomePage
