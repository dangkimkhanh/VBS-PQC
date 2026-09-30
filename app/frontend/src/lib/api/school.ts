import apiService from './root'

export interface MySchool {
  university_name: string
  university_code: string
  email_domain: string
}

// The signed-in school admin's own university.
export const getMySchool = async (): Promise<MySchool> => {
  const res = await apiService('GET', 'school/profile')
  return res.data
}
