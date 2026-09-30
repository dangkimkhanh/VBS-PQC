import { DegreeTemplateType, OptionType } from '@/types/common'
import { format, parse, parseISO } from 'date-fns'

export const formatStudent = (data: any, isSendToServer: boolean = false) => {
  return isSendToServer
    ? {
        student_code: data.code,
        full_name: data.name,
        email: data.email,
        faculty_code: data.faculty,
        course: isNaN(Number(data.year)) ? '' : String(data.year),
        citizen_id_number: data.citizenId,
        ethnicity: data.ethnicity,
        current_address: data.currentAddress,
        birth_address: data.birthAddress,
        union_join_date: data.unionJoinDate ? format(new Date(data.unionJoinDate), 'dd/MM/yyyy') : undefined,
        party_join_date: data.partyJoinDate ? format(new Date(data.partyJoinDate), 'dd/MM/yyyy') : undefined,
        description: data.description,
        date_of_birth: data.dateOfBirth ? format(new Date(data.dateOfBirth), 'dd/MM/yyyy') : undefined,
        gender: Boolean(data.gender)
      }
    : {
        id: data.id,
        code: data.student_code,
        name: data.full_name,
        email: data.email,
        faculty: data.faculty_code,
        facultyName: data.faculty_name,
        year: data.course,
        status: String(data.status),
        citizenId: data.citizen_id_number,
        ethnicity: data.ethnicity,
        currentAddress: data.current_address,
        birthAddress: data.birth_address,
        unionJoinDate: formatDateForInput(data.union_join_date),
        partyJoinDate: formatDateForInput(data.party_join_date),
        description: data.description,
        dateOfBirth: formatDateForInput(data.date_of_birth),
        gender: String(data.gender)
      }
}

export const formatFaculty = (data: any, isSendToServer: boolean = false) => {
  return isSendToServer
    ? {
        faculty_code: data.code,
        faculty_name: data.name,
        description: data.description
      }
    : {
        id: data.id,
        code: data.faculty_code,
        name: data.faculty_name,
        description: data.description
      }
}

export const formatFacultyOptions = (data: any) => {
  return data.map((item: any) => ({
    label: item.name,
    value: item.code
  }))
}

export const formatFacultyOptionsByID = (data: any) => {
  return data.map((item: any) => ({
    label: item.name,
    value: item.id
  }))
}

export const formatDegreeTemplateOptions = (data: any) => {
  return data.map((item: any) => ({
    label: item.name,
    value: item.id
  }))
}

export const formatCertificate = (data: any, isDegree: boolean, isSendToServer: boolean = false) => {
  return isSendToServer
    ? isDegree
      ? {
          student_code: data.studentCode,
          name: data.name,
          certificate_type: data.certificateType,
          serial_number: data.serialNumber,
          reg_no: data.regNo,
          issue_date: new Date(data.date),
          major: data.major,
          graduation_rank: data.graduationRank,
          gpa: data.gpa,
          description: data.description,
          is_degree: true,
          course: data.course,
          education_type: data.educationType
        }
      : {
          student_code: data.studentCode,
          name: data.name,
          serial_number: data.serialNumber,
          reg_no: data.regNo,
          issue_date: new Date(data.date),
          is_degree: false,
          description: data.description
        }
    : {
        id: data.id,
        studentCode: data.student_code,
        studentName: data.student_name,
        faculty: data.faculty_code,
        facultyName: data.faculty_name,
        certificateType: data.certificate_type,
        date: data.issue_date,
        signed: data.signed,
        name: data.name,
        isDegree: data.graduation_rank !== undefined,
        onBlockchain: data.on_blockchain,
        universityCode: data.university_code,
        universityId: data.university_id,
        course: data.course,
        educationType: data.education_type,
        onBlockchainVerify: data.on_blockchain_verify,
        graduationRank: data.graduation_rank,
        gpa: data.gpa,
        serialNumber: data.serial_number,
        regNo: data.reg_no
      }
}

export const formatCertificateView = (data: any) => {
  return {
    studentCode: data.student_code,
    studentName: data.student_name,
    facultyCode: data.faculty_code,
    facultyName: data.faculty_name,
    certificateType: data.certificate_type,
    date: data.issue_date,
    name: data.name,
    universityName: data.university_name,
    universityCode: data.university_code,
    serialNumber: data.serial_number,
    regNo: data.reg_no,
    signed: data.signed,
    description: data.description,
    gpa: data.gpa,
    dateOfBirth: data.date_of_birth,
    course: data.course,
    graduationRank: data.graduation_rank,
    major: data.major || data.faculty_name,
    educationType: data.education_type
  }
}

export const formatCertificateVerifyCode = (data: any, isSendToServer: boolean = false) => {
  return isSendToServer
    ? {
        duration_minutes: data.expiredAfter,
        can_view_score: data.permissionType.includes('can_view_score'),
        can_view_data: data.permissionType.includes('can_view_data'),
        can_view_file: data.permissionType.includes('can_view_file')
      }
    : {
        verifyCode: data.code,
        createdAt: format(new Date(data.created_at), 'dd/MM/yyyy HH:mm:ss'),
        expiredAfter: data.expired_in_minutes,
        permissionType: [
          data.can_view_score ? 'can_view_score' : null,
          data.can_view_data ? 'can_view_data' : null,
          data.can_view_file ? 'can_view_file' : null
        ].filter(Boolean) as ('can_view_score' | 'can_view_data' | 'can_view_file')[],
        status: data.expired_in_minutes !== 0
      }
}

const getIsDiscipline = (data: any) => {
  switch (data) {
    case 'true':
      return true
    case 'false':
      return false
    case undefined:
      return undefined
    case '':
      return undefined
    default:
      return undefined
  }
}

export const formatRewardDiscipline = (data: any, isSendToServer: boolean = false) => {
  return isSendToServer
    ? {
        student_code: data.studentCode,
        name: data.name,
        decision_number: data.decisionNumber,
        is_discipline: !!data.disciplineLevel || getIsDiscipline(data.isDiscipline),
        description: data.description,
        discipline_level: data.disciplineLevel ? Number(data.disciplineLevel) : undefined
      }
    : {
        id: data.id,
        name: data.name,
        studentCode: data.student_code,
        studentName: data.student_name,
        faculty: data.faculty_code,
        facultyName: data.faculty_name,
        decisionNumber: data.decision_number,
        description: data.description,
        isDiscipline: data.is_discipline,
        disciplineLevel: String(data.discipline_level),
        createdAt: format(new Date(data.created_at), 'dd/MM/yyyy HH:mm:ss')
      }
}

export const formatDegreeTemplateFormData = (data: DegreeTemplateType, isCreate: boolean = true) => {
  const formData = new FormData()

  formData.append('name', data.name)
  if (data.description) formData.append('description', data.description)
  if (isCreate) formData.append('faculty_id', data.faculty_id)
  formData.append('html_content', data.html_content)

  return formData
}

// Preview copy of the digital-signature stamp drawn by the backend
// (service/diploma_seal.go) for "Trường Đại học Mẫu" signed with ML-DSA-65.
const SAMPLE_SEAL_SVG =
  '<svg class="seal" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">' +
  '<defs><path id="seal-top" d="M 34,100 A 66,66 0 0 1 166,100"/><path id="seal-bottom" d="M 22,100 A 78,78 0 0 0 178,100"/></defs>' +
  '<g fill="none" stroke="#1e3a8a"><circle cx="100" cy="100" r="95" stroke-width="5"/><circle cx="100" cy="100" r="86" stroke-width="1.5"/><circle cx="100" cy="100" r="55" stroke-width="1.5"/></g>' +
  '<g fill="#1e3a8a" font-family="Arial, Helvetica, sans-serif" font-weight="700" text-anchor="middle">' +
  '<text font-size="14" letter-spacing="1"><textPath href="#seal-top" startOffset="50%">KÝ SỐ HẬU LƯỢNG TỬ</textPath></text>' +
  '<text font-size="15"><textPath href="#seal-bottom" startOffset="50%">TRƯỜNG ĐẠI HỌC MẪU</textPath></text>' +
  '<polygon points="22.0,94.0 23.8,98.1 27.7,98.1 24.6,100.8 25.6,105.0 22.0,102.6 18.4,105.0 19.4,100.8 16.3,98.1 20.2,98.1"/>' +
  '<polygon points="178.0,94.0 179.8,98.1 183.7,98.1 180.6,100.8 181.6,105.0 178.0,102.6 174.4,105.0 175.4,100.8 172.3,98.1 176.2,98.1"/>' +
  '<text x="100" y="107" font-size="19">ML-DSA-65</text></g></svg>'

// Sample values for previewing a diploma layout; keys match the data the
// backend passes to the template (service/diploma_render.go).
const TEMPLATE_PREVIEW_VALUES: Record<string, string> = {
  TenTruong: 'Trường Đại học Mẫu',
  LoaiVanBang: 'BẰNG KỸ SƯ',
  Nganh: 'Công nghệ thông tin',
  HoTen: 'Nguyễn Văn A',
  NgaySinh: '01/01/2003',
  NgayCap: '01/01/2026',
  NgayCapChu: 'Ngày 01 tháng 01 năm 2026',
  SoHieu: 'CT060999',
  SoVaoSo: '1234567890',
  HinhThucDaoTao: 'Chính quy',
  XepLoai: 'Giỏi',
  Khoa: '2021',
  ChucDanhKy: 'HIỆU TRƯỞNG',
  TenNguoiKy: 'Nguyễn Văn B',
  ConDau: SAMPLE_SEAL_SVG
}

export const formatTemplateInterfaceHTML = (html: string) =>
  html
    // Every sample value is present, so conditional blocks are shown as-is.
    .replace(/\{\{-?\s*(if|else|end)\b[^}]*\}\}/g, '')
    .replace(/\{\{-?\s*\.(\w+)\s*-?\}\}/g, (match, key: string) => TEMPLATE_PREVIEW_VALUES[key] ?? match)

export const formatTemplateInterfaceOptions = (data: any): OptionType[] => {
  return data.map((item: any) => ({
    value: item.id,
    label: item.name
  }))
}

export const formatTinyTextEdit = (html: string) => {
  return html.includes('<html lang="vi">')
    ? html
    : `<!DOCTYPE html>
<html lang="vi">
<head>
  <meta charset="UTF-8">
  <title>Văn bằng</title>
  <style>
    body {
      margin: 0;
      padding: 0;
      background-color: #f0f0f0;
      font-family: Arial, sans-serif;
    }
  </style>
</head>
<body> ${html} </body>
</html>`
}

export const formatDateForInput = (rawDate: string): string => {
  try {
    const dateObj = parse(rawDate, 'dd/MM/yyyy', new Date())
    return format(dateObj, 'yyyy-MM-dd')
  } catch {
    return ''
  }
}

export const formatDateISO = (input: string): string => {
  try {
    if (!input) return ''

    const date = parseISO(input)
    if (isNaN(date.getTime())) {
      // Trường hợp date không hợp lệ
      return ''
    }

    return format(date, 'dd/MM/yyyy')
  } catch {
    return ''
  }
}
