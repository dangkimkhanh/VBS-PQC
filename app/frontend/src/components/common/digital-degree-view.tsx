'use client'

import useSWR from 'swr'
import DecriptionView from './description-view'
import {
  AlertTriangle,
  Book,
  BookOpen,
  Calendar,
  ChartAreaIcon,
  Download,
  FileTextIcon,
  Library,
  RefreshCw,
  School,
  ShieldCheck,
  TagsIcon,
  Text,
  User
} from 'lucide-react'
import PDFView from './pdf-view'

// Saves the already-loaded PDF under a readable name. The browser PDF viewer's
// own download button saves blob files under a random id.
const downloadBlob = (blob: Blob, filename: string) => {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  link.remove()
  // Revoke later: the download reads the blob asynchronously.
  setTimeout(() => URL.revokeObjectURL(url), 60_000)
}
import { Separator } from '../ui/separator'
import { Alert, AlertDescription, AlertTitle } from '../ui/alert'
import { Button } from '../ui/button'
import { Badge } from '../ui/badge'
import { showNotification } from '@/lib/utils/common'
import { getDigitalDegreeById, getDigitalDegreeFileById } from '@/lib/api/digital-degree'
import { verifyPQCEDiploma } from '@/lib/api/pqc'
import DigitalDegreeQR from './digital-degree-qr'

interface Props {
  id: string
}

const BLOCKCHAIN_STATUS: Record<string, string> = {
  verified: 'Đã xác nhận',
  not_anchored: 'Chưa ghi Blockchain',
  not_checked: 'Chưa kiểm tra',
  unavailable: 'Không kết nối được Blockchain',
  mismatch: 'Không khớp dữ liệu trên Blockchain',
  inconsistent: 'Dữ liệu neo không nhất quán',
  invalid_local_proof: 'Bằng chứng Merkle không hợp lệ',
  missing_pqc_transaction: 'Thiếu chữ ký giao dịch PQC',
  legacy_batch: 'Lô ghi theo định dạng cũ'
}
const blockchainStatusLabel = (status?: string) => (status && BLOCKCHAIN_STATUS[status]) || status || 'Không xác định'

// Authenticated view of one diploma, used by both the university admin and
// the student who owns it. Public verification lives at /verify/[id].
const DigitalDegreeView: React.FC<Props> = ({ id }) => {
  const queryData = useSWR(id ? `digital-degree-view-${id}` : undefined, () => getDigitalDegreeById(id), {
    onError: (error) => showNotification('error', error.message || 'Không tải được dữ liệu văn bằng')
  })
  const degree = queryData.data?.data

  const queryFile = useSWR(
    degree?.issued ? `digital-degree-file-${id}` : undefined,
    () => getDigitalDegreeFileById(id),
    {
      revalidateOnFocus: false,
      shouldRetryOnError: false,
      onError: () => showNotification('error', 'Không tải được tệp PDF')
    }
  )

  const queryPQC = useSWR(degree?.pqc_proof ? `pqc-verification-${id}` : undefined, () => verifyPQCEDiploma(id), {
    shouldRetryOnError: false
  })
  const pqc = queryPQC.data

  const items = [
    {
      icon: <School className='h-5 w-5 text-gray-500' />,
      title: 'Đơn vị đào tạo',
      value: `${degree?.university_code ?? ''} - ${degree?.university_name ?? ''}`
    },
    {
      icon: <User className='h-5 w-5 text-gray-500' />,
      title: 'Sinh viên',
      value: `${degree?.student_code ?? ''} - ${degree?.student_name ?? ''}`
    },
    {
      icon: <Library className='h-5 w-5 text-gray-500' />,
      title: 'Ngành học',
      value: `${degree?.faculty_code ?? ''} - ${degree?.faculty_name ?? ''}`
    },
    { icon: <BookOpen className='h-5 w-5 text-gray-500' />, title: 'Hệ đào tạo', value: degree?.education_type },
    {
      icon: <Book className='h-5 w-5 text-gray-500' />,
      title: 'Văn bằng',
      value: (
        <div>
          <Badge className='bg-blue-500 text-white hover:bg-blue-400'>{degree?.certificate_type ?? '-'}</Badge>
          {' - '}
          <span>{degree?.name}</span>
        </div>
      )
    },
    { icon: <ChartAreaIcon className='h-5 w-5 text-gray-500' />, title: 'GPA', value: degree?.gpa },
    { icon: <Calendar className='h-5 w-5 text-gray-500' />, title: 'Ngày cấp', value: degree?.issue_date },
    { icon: <TagsIcon className='h-5 w-5 text-gray-500' />, title: 'Số hiệu', value: degree?.serial_number },
    {
      icon: <FileTextIcon className='h-5 w-5 text-gray-500' />,
      title: 'Số vào sổ gốc cấp văn bằng',
      value: degree?.registration_number
    },
    { icon: <Text className='h-5 w-5 text-gray-500' />, title: 'Xếp loại', value: degree?.graduation_rank }
  ]

  return (
    <div>
      {degree?.revoked && (
        <Alert variant='destructive' className='mb-4'>
          <AlertTriangle />
          <AlertTitle>Văn bằng đã thu hồi</AlertTitle>
          <AlertDescription>
            {degree.revocation_reason}
            {degree.revoked_at && ` · ${new Date(degree.revoked_at).toLocaleString('vi-VN')}`}
          </AlertDescription>
        </Alert>
      )}
      {degree?.replaces_id && (
        <p className='my-4 rounded-xl border bg-slate-50 p-4 text-sm dark:bg-slate-900'>
          Văn bằng thay thế cho hồ sơ: {degree.replaces_id}
        </p>
      )}
      {degree && !degree.pqc_proof && (
        <Alert className='mx-auto mb-4 max-w-[800px]'>
          <AlertTriangle />
          <AlertTitle>{degree.issued ? 'Văn bằng chưa được ký ML-DSA' : 'Văn bằng chưa được cấp'}</AlertTitle>
          <AlertDescription>
            Văn bằng chỉ có giá trị xác minh sau khi được cấp PDF, ký ML-DSA và ghi nhận trên Blockchain.
          </AlertDescription>
        </Alert>
      )}
      {pqc && (
        <Alert
          className='mx-auto mb-4 max-w-[800px]'
          variant={
            pqc.assurance_level === 'complete'
              ? 'success'
              : pqc.assurance_level === 'signature_only'
                ? 'default'
                : 'destructive'
          }
        >
          {pqc.valid ? <ShieldCheck /> : <AlertTriangle />}
          <AlertTitle>
            {pqc.assurance_level === 'complete'
              ? 'Văn bằng được xác minh đầy đủ'
              : pqc.assurance_level === 'signature_only'
                ? 'Chữ ký hợp lệ, chưa được Blockchain xác nhận'
                : 'Xác minh văn bằng không hợp lệ'}
          </AlertTitle>
          <AlertDescription>
            <div>Thuật toán: {pqc.algorithm}</div>
            <div>Chữ ký và khóa tại thời điểm ký: {pqc.signature_valid ? 'Hợp lệ' : 'Không hợp lệ'}</div>
            <div>
              Tệp PDF:{' '}
              {pqc.file_integrity_checked
                ? pqc.file_integrity_valid
                  ? 'Nguyên vẹn'
                  : 'Đã thay đổi'
                : 'Không thể kiểm tra'}
            </div>
            <div>Blockchain: {blockchainStatusLabel(pqc.blockchain?.status)}</div>
            {pqc.blockchain?.batch_id && <div className='break-all'>Mã khối phát hành: {pqc.blockchain.batch_id}</div>}
            {pqc.warning && <div>{pqc.warning}</div>}
            {pqc.error && <div>{pqc.error}</div>}
          </AlertDescription>
        </Alert>
      )}

      <DecriptionView
        title={degree?.name || (queryData.isLoading ? 'Đang tải…' : 'Không có dữ liệu')}
        items={items}
        description='Thông tin chi tiết về văn bằng số'
        extra={degree?.issued && !degree?.revoked ? <DigitalDegreeQR id={id} name={degree?.name} /> : undefined}
      />

      {queryFile.data ? (
        <>
          <Separator className='my-3' />
          <div className='flex items-center justify-between'>
            <h3 className='mb-3'>Tệp PDF</h3>
            <div className='flex gap-2'>
              <Button
                variant='outline'
                onClick={() =>
                  downloadBlob(
                    queryFile.data as Blob,
                    `van-bang-${degree?.student_code || degree?.serial_number || id}.pdf`
                  )
                }
              >
                <Download /> Tải PDF
              </Button>
              <Button size='icon' variant='outline' title='Tải lại tệp' onClick={() => queryFile.mutate()}>
                <RefreshCw />
              </Button>
            </div>
          </div>
          <div className='mt-4 h-[700px]'>
            <PDFView url={queryFile.data} loading={queryFile.isLoading} />
          </div>
        </>
      ) : (
        degree && (
          <p className='mt-4 text-center text-muted-foreground'>
            {degree.issued ? 'Đang tải tệp PDF…' : 'Chưa có tệp PDF'}
          </p>
        )
      )}
    </div>
  )
}

export default DigitalDegreeView
