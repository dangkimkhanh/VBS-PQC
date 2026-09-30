'use client'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { useState } from 'react'
import { PackageSearch } from 'lucide-react'
import { useRouter } from 'next/navigation'
import ImgVerifyButton from './img-verify-button'
import { showMessage } from '@/lib/utils/common'

// Accepts the diploma verification code or a pasted verification link.
const parseCode = (input: string) => {
  const value = input.trim()
  const fromLink = value.match(/\/verify\/([^/?#]+)/)
  return fromLink ? decodeURIComponent(fromLink[1]) : value
}

const SearchVerifyCode = () => {
  const [code, setCode] = useState('')
  const router = useRouter()

  const openVerification = (raw: string) => {
    const value = parseCode(raw)
    if (!/^[a-f0-9]{24}$/i.test(value)) {
      showMessage('Mã xác minh không hợp lệ. Vui lòng kiểm tra lại mã in trên văn bằng hoặc quét mã QR.')
      return
    }
    router.push(`/verify/${value}`)
  }

  return (
    <>
      <ImgVerifyButton onCodeDetected={openVerification} />
      <p className='my-4 text-center text-muted-foreground'>hoặc</p>
      <Card className='mb-6 w-full max-w-[600px]'>
        <CardHeader>
          <CardTitle className='px-3 text-center md:px-6'>
            <h3>Nhập mã xác minh văn bằng số</h3>
          </CardTitle>
        </CardHeader>
        <CardContent className='px-3 md:px-6'>
          <form
            className='flex items-center gap-2'
            onSubmit={(event) => {
              event.preventDefault()
              openVerification(code)
            }}
          >
            <Input
              aria-label='Mã xác minh văn bằng'
              placeholder='Mã xác minh hoặc liên kết xác minh'
              value={code}
              onChange={(e) => setCode(e.target.value)}
            />
            <Button type='submit' disabled={!code.trim()}>
              <PackageSearch /> <span className='hidden md:block'>Xác minh</span>
            </Button>
          </form>
        </CardContent>
      </Card>
    </>
  )
}

export default SearchVerifyCode
