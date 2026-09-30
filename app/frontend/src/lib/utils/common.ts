import { ExternalToast, toast } from 'sonner'
import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'
import { OptionType } from '@/types/common'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

// eslint-disable-next-line no-unused-vars
export function debounce<T extends (...args: any[]) => void>(func: T, wait: number): (...args: Parameters<T>) => void {
  let timeout: ReturnType<typeof setTimeout> | null = null

  return (...args: Parameters<T>) => {
    if (timeout !== null) {
      clearTimeout(timeout)
    }
    timeout = setTimeout(() => {
      func(...args)
    }, wait)
  }
}

export const clearFalsyValueObject = (obj: Record<string, any>) => {
  return Object.fromEntries(Object.entries(obj).filter((entry) => entry[1] !== null && entry[1] !== undefined))
}

export const queryString = (slashParams: (string | number)[], params?: any) => {
  const filteredParams = params ? clearFalsyValueObject(params) : null
  const queryString = filteredParams ? new URLSearchParams(filteredParams as Record<string, string>).toString() : null
  return `${slashParams.join('/')}${queryString ? '?' + queryString : ''}`
}

// Backend messages that are technical or English, mapped to wording for end users.
// Matching is by lowercase substring; the first match wins.
const ERROR_TRANSLATIONS: [string, string][] = [
  ['no unsigned, issued diplomas match', 'Không có văn bằng đã cấp nào chờ ký trong bộ lọc hiện tại.'],
  ['no active ml-dsa key', 'Chưa có khóa ký đang hoạt động. Vui lòng tạo và kích hoạt khóa trước khi ký.'],
  ['faculty_id is required', 'Vui lòng chọn chuyên ngành trong bộ lọc.'],
  ['pqc_master_key is not configured', 'Máy chủ chưa được cấu hình để ký văn bằng. Vui lòng liên hệ quản trị hệ thống.'],
  ['pqc transaction', 'Blockchain từ chối chữ ký xác nhận của lô văn bằng. Vui lòng ký lại hoặc liên hệ quản trị hệ thống.'],
  ['public key fingerprint mismatch', 'Blockchain từ chối chữ ký xác nhận của lô văn bằng. Vui lòng ký lại hoặc liên hệ quản trị hệ thống.'],
  ['no valid ediplomas to push', 'Không có văn bằng nào đã cấp và ký để ghi lên Blockchain.'],
  ['anchored on blockchain cannot be re-signed', 'Văn bằng đã ghi lên Blockchain nên không thể ký lại.'],
  ['diploma already has a pqc proof', 'Văn bằng này đã được ký.'],
  ['must be issued and have a file hash', 'Văn bằng cần được cấp PDF trước khi ký.'],
  ['revoked diploma cannot be signed', 'Không thể ký văn bằng đã thu hồi.'],
  ['only issued diplomas can be revoked', 'Chỉ có thể thu hồi văn bằng đã cấp.'],
  ['already revoked', 'Văn bằng này đã được thu hồi.'],
  ['revocation reason must', 'Lý do thu hồi cần ít nhất 5 ký tự.'],
  ['reason must be between', 'Lý do thu hồi cần từ 5 đến 500 ký tự.'],
  ['compromise effective time cannot be in the future', 'Thời điểm lộ khóa không được ở tương lai.'],
  ['revoked keys cannot be activated', 'Không thể kích hoạt khóa đã thu hồi.'],
  ['only pending keys can be activated', 'Chỉ có thể kích hoạt khóa đang chờ.'],
  ['key name is required', 'Vui lòng nhập tên khóa.'],
  ['template is locked', 'Mẫu bằng đã được dùng để cấp văn bằng nên không thể sửa.'],
  ['template does not belong', 'Mẫu bằng không thuộc chuyên ngành đã chọn.'],
  ['template sample not found', 'Không tìm thấy giao diện mẫu.'],
  ['template not found', 'Không tìm thấy mẫu bằng.'],
  ['ediploma not found', 'Không tìm thấy văn bằng.'],
  ['access denied', 'Bạn không có quyền thao tác với dữ liệu này.'],
  ['does not belong', 'Bạn không có quyền thao tác với dữ liệu này.'],
  ['unauthorized', 'Phiên đăng nhập không hợp lệ. Vui lòng đăng nhập lại.']
]

const GENERIC_ERROR = 'Thao tác không thành công. Vui lòng thử lại.'

// Messages without any Vietnamese letters come from internals and are never shown verbatim.
const isTechnicalMessage = (message: string) => !/[À-ỹĐđ]/.test(message)

export const getUserFriendlyErrorMessage = (error: unknown, fallback = GENERIC_ERROR) => {
  const rawMessage =
    typeof error === 'string'
      ? error
      : error && typeof error === 'object' && 'message' in error
        ? String((error as { message?: unknown }).message || '')
        : ''
  const message = rawMessage.toLowerCase()
  const translated = ERROR_TRANSLATIONS.find(([pattern]) => message.includes(pattern))
  if (translated) return translated[1]
  if (!rawMessage || isTechnicalMessage(rawMessage)) return fallback
  return rawMessage
}

const NOTIFICATION_TITLES = {
  success: 'Thành công',
  error: 'Không thành công',
  info: 'Thông tin',
  warning: 'Lưu ý',
  message: 'Thông báo'
}

export const showNotification = (
  type: 'success' | 'error' | 'info' | 'warning' | 'message',
  description: string,
  setting?: ExternalToast
) => {
  // Only failures go through the error translator; a success message is never
  // replaced by an error text, it just falls back to the plain title.
  const text =
    type === 'error' || type === 'warning'
      ? getUserFriendlyErrorMessage(description)
      : description && !isTechnicalMessage(description)
        ? description
        : ''
  return toast[type](NOTIFICATION_TITLES[type], {
    description: text || undefined,
    classNames: {
      success: '[&_svg]:!text-green-500',
      error: '[&_svg]:!text-red-500',
      info: '[&_svg]:!text-blue-500',
      warning: '[&_svg]:!text-yellow-500'
    },
    ...setting
  })
}

export const showMessage = (description: string, setting?: ExternalToast) => {
  return toast(description, {
    position: 'top-center',
    ...setting
  })
}

export const formatResponseImportExcel = (
  data: any
): {
  success: number[]
  error: { title: string; row: number[] }[]
} => {
  const successRows = (data?.data?.success ?? []).map((item: any) => item.row)
  const errorMap = (data?.data?.error ?? []).reduce(
    (acc: Record<string, { title: string; row: number[] }>, item: any) => {
      if (!acc[item.error]) {
        acc[item.error] = { title: item.error, row: [] }
      }
      acc[item.error].row.push(item.row)
      return acc
    },
    {}
  )

  return {
    success: successRows,
    error: Object.values(errorMap)
  }
}

export const formatBytes = (bytes: number) => {
  if (!bytes && bytes !== 0) return ''
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  const value = bytes / Math.pow(k, i)
  const digits = value >= 100 ? 0 : value >= 10 ? 1 : 2
  return `${value.toFixed(digits)} ${sizes[i]}`
}

export const findLabel = (id: string, options: OptionType[]) => {
  return options?.find((o: OptionType) => o.value === id)?.label
}

export const extractBodyInnerHTML = (html: string) => {
  const match = html.match(/<body[^>]*>([\s\S]*?)<\/body>/i)
  return match ? match[1] : html
}

export const isFullDocument = (html: string) => /<\s*html[\s>]/i.test(html)
