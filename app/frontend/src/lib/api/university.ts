import { queryString } from '../utils/common'
import apiService from './root'

export type UniversityPayload = Record<string, string>

export type UniversityStatusFilter = '' | 'active' | 'pending' | 'locked'

export interface UniversityListParams {
  q?: string
  status?: UniversityStatusFilter
  page?: number
  page_size?: number
}

// Returns { data, total, page, page_size, total_page }.
export const getUniversityList = async (params: UniversityListParams = {}) =>
  apiService('GET', queryString(['universities'], params))

export const getAdminOverview = async () => {
  const res = await apiService('GET', 'admin/overview')
  return res.data
}

export const getAdminAudit = async () => {
  const res = await apiService('GET', 'admin/audit')
  return res.data
}

export const getUniversity = async (id: string) => {
  const res = await apiService('GET', `universities/${id}`)
  return res.data
}

export const createUniversity = async (payload: UniversityPayload) => apiService('POST', 'universities', payload)

export const updateUniversity = async (id: string, payload: UniversityPayload) =>
  apiService('PUT', `universities/${id}`, payload)

export const lockUniversity = async (id: string) => apiService('POST', `universities/${id}/lock`)

export const unlockUniversity = async (id: string) => apiService('POST', `universities/${id}/unlock`)

export const resendUniversityActivation = async (id: string) =>
  apiService('POST', `universities/${id}/resend-activation`)
