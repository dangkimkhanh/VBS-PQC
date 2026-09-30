'use client'
import { Book, School, User, Mail, Library, Calendar, AwardIcon } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import DecriptionView from '@/components/common/description-view'
import useSWR from 'swr'
import { getStudentInformation } from '@/lib/api/student'

const getStudentInfoItems = (data: any) => [
  {
    icon: <School className='h-5 w-5 text-gray-500' />,
    title: 'Đơn vị cấp bằng',
    value: `${data?.universityCode} - ${data?.univeristyName}`
  },
  {
    icon: <User className='h-5 w-5 text-gray-500' />,
    title: 'Họ và tên',
    value: data?.name
  },
  {
    icon: <Book className='h-5 w-5 text-gray-500' />,
    title: 'Mã sinh viên',
    value: data?.code
  },
  {
    icon: <Mail className='h-5 w-5 text-gray-500' />,
    title: 'Email',
    value: data?.email
  },
  {
    icon: <Library className='h-5 w-5 text-gray-500' />,
    title: 'Ngành học',
    value: `${data?.facultyCode} - ${data?.facultyName}`
  },
  {
    icon: <Calendar className='h-5 w-5 text-gray-500' />,
    title: 'Năm nhập học',
    value: data?.year
  },
  {
    icon: <AwardIcon className='h-5 w-5 text-gray-500' />,
    title: 'Trạng thái',
    value: (
      <Badge variant={Number(data?.status) === 1 ? 'default' : 'outline'}>
        {Number(data?.status) === 1 ? 'Đã tốt nghiệp' : 'Chưa tốt nghiệp'}
      </Badge>
    )
  }
]

export default function StudentDashboard() {
  const queryData = useSWR('student-information', () => getStudentInformation())

  return (
    <div>
      <h2>Thông tin cá nhân</h2>
      <p className='mb-4 mt-1 text-sm text-muted-foreground'>
        Hồ sơ do trường quản lý, bạn chỉ có quyền xem. Nếu thông tin sai, vui lòng liên hệ phòng đào tạo.
      </p>
      <DecriptionView
        title='Thông tin cá nhân'
        description='Thông tin chi tiết về hồ sơ sinh viên'
        items={getStudentInfoItems(queryData.data || {})}
      />
    </div>
  )
}
