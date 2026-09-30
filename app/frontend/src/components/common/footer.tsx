import Image from 'next/image'
import Link from 'next/link'
import appLogo from '../../../public/assets/images/applogo.png'

const Footer: React.FC = () => {
  return (
    <footer className='w-full border-t bg-muted/40 py-6'>
      <div className='container flex flex-col gap-3 text-sm text-muted-foreground md:flex-row md:items-center md:justify-between'>
        <Link href='/' className='flex items-center gap-2'>
          <Image src={appLogo} alt='Văn bằng số PQC' width={24} height={24} />
          <span className='font-semibold text-main'>Văn bằng số PQC</span>
        </Link>
        <nav className='flex flex-wrap gap-x-5 gap-y-1'>
          <Link className='hover:text-foreground' href='/'>
            Xác minh văn bằng
          </Link>
          <Link className='hover:text-foreground' href='/auth/university-contact'>
            Dành cho cơ sở đào tạo
          </Link>
        </nav>
        <p>© {new Date().getFullYear()} Văn bằng số PQC</p>
      </div>
    </footer>
  )
}

export default Footer
