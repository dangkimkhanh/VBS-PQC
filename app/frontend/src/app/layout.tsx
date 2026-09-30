import type { Metadata } from 'next'
import { Geist, Geist_Mono } from 'next/font/google'
import { ThemeProvider } from '@/components/providers/theme-provider'
import SWRConfig from '@/components/providers/swr-config'
import { Toaster } from 'sonner'
import '../../public/assets/styles/global.css'

const geistSans = Geist({
  variable: '--font-geist-sans',
  subsets: ['latin']
})

const geistMono = Geist_Mono({
  variable: '--font-geist-mono',
  subsets: ['latin']
})

export const metadata: Metadata = {
  title: 'Văn bằng số PQC',
  description: 'Hệ thống cấp phát và quản lý văn bằng số ứng dụng Blockchain.',
  icons: {
    icon: '/assets/images/applogo.png'
  }
}

export default async function RootLayout({
  children
}: Readonly<{
  children: React.ReactNode
}>) {
  return (
    <html lang={'vi'} suppressHydrationWarning>
      <body className={`${geistSans.variable} ${geistMono.variable} antialiased`}>
        <ThemeProvider
          attribute='class'
          defaultTheme='system'
          enableSystem
          disableTransitionOnChange
          storageKey='theme'
        >
          <SWRConfig>
            {children}
            <Toaster expand={true} />
          </SWRConfig>
        </ThemeProvider>
      </body>
    </html>
  )
}
