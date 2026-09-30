import { z } from 'zod'

export const validateEmail = z
  .string()
  .trim()
  .nonempty({
    message: 'Email không được để trống'
  })
  .email({
    message: 'Email không hợp lệ (VD: example@gmail.com)'
  })

export const validatePassword = z.string().trim().nonempty({
  message: 'Mật khẩu không được để trống'
})

// Must match the backend rule (bcrypt accepts at most 72 bytes).
export const validateNewPassword = z
  .string()
  .min(8, { message: 'Mật khẩu phải có ít nhất 8 ký tự' })
  .max(72, { message: 'Mật khẩu tối đa 72 ký tự' })
// The school's own email domain is enforced by the backend, which knows it;
// here we only check that the address is well formed.
export const validateAcademicEmail = z
  .string()
  .trim()
  .nonempty({
    message: 'Email không được để trống'
  })
  .email({
    message: 'Email không hợp lệ'
  })

export const validateNoEmpty = (name: string) => {
  return z
    .string()
    .trim()
    .nonempty({
      message: `${name} không được để trống`
    })
}

export const validateCitizenId = z
  .string()
  .trim()
  .nonempty({
    message: 'Số CCCD không được để trống'
  })
  .min(12, {
    message: 'Số CCCD phải có 12 chữ số'
  })
  .regex(/^\d+$/, {
    message: 'Số CCCD chỉ gồm chữ số'
  })

export const validateGPA = z
  .number()
  .or(z.string().transform((val) => parseFloat(val)))
  .refine((val) => val >= 0, {
    message: 'Điểm GPA phải lớn hơn 0'
  })
  .refine((val) => val <= 10, {
    message: 'Điểm GPA phải nhỏ hơn 10'
  })
