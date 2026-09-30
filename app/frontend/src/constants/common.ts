import { searchStudentByCode } from '@/lib/api/student'

export const STUDENT_STATUS_OPTIONS = [
  {
    label: 'Đã tốt nghiệp',
    value: '1'
  },
  {
    label: 'Chưa tốt nghiệp',
    value: '0'
  }
]

export const GENDER_SELECT_SETTING = {
  select: {
    groups: [
      {
        label: undefined,
        options: [
          { label: 'Nam', value: 'true' },
          { label: 'Nữ', value: 'false' }
        ]
      }
    ]
  }
}

export const CERTIFICATE_TYPE_OPTIONS = [
  { value: 'Cử nhân', label: 'Cử nhân' },
  { value: 'Kỹ sư', label: 'Kỹ sư' },
  { value: 'Thạc sĩ', label: 'Thạc sĩ' },
  { value: 'Tiến sĩ', label: 'Tiến sĩ' }
]

export const STUDENT_CODE_SEARCH_SETTING = {
  querySelect: {
    queryFn: (keyword: string) => searchStudentByCode(keyword)
  }
}
export const PAGE_SIZE = 10

export const GRADUATION_RANK_OPTIONS = [
  { value: 'Giỏi', label: 'Giỏi' },
  { value: 'Khá', label: 'Khá' },
  { value: 'Trung bình', label: 'Trung bình' },
  { value: 'Yếu', label: 'Yếu' }
]
