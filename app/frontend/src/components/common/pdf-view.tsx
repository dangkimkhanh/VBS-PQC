import { Loader2 } from 'lucide-react'
import { useEffect, useMemo } from 'react'

interface Props {
  url: string | undefined
  loading: boolean
}

const PDFView: React.FC<Props> = (props) => {
  const file = props.url as unknown as Blob | undefined
  const fileUrl = useMemo(() => (file ? URL.createObjectURL(file) : undefined), [file])

  useEffect(() => {
    return () => {
      // Delay the revoke so a download started from the viewer can finish.
      if (fileUrl) setTimeout(() => URL.revokeObjectURL(fileUrl), 60_000)
    }
  }, [fileUrl])

  if (!fileUrl && !props.loading)
    return (
      <div className='h-full w-full'>
        <p className='text-center text-red-500'> Không có tệp PDF</p>
      </div>
    )
  if (props.loading)
    return (
      <div className='flex h-full w-full items-center justify-center'>
        <Loader2 className='h-4 w-4 animate-spin' />
        <p className='text-center text-sm text-gray-500'>Đang tải file PDF...</p>
      </div>
    )

  return (
    <div className='h-full min-h-[500px] w-full'>
      {/* view=Fit shows the whole page, so a landscape diploma needs no scrolling. */}
      <iframe src={`${fileUrl}#view=Fit`} className='h-full w-full' title='Tệp PDF văn bằng' />
    </div>
  )
}

export default PDFView
