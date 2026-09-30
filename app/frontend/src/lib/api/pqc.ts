import apiService from './root'

export type PQCKeyStatus = 'pending' | 'active' | 'retired' | 'revoked'

export type PQCAlgorithm = 'ML-DSA-44' | 'ML-DSA-65' | 'ML-DSA-87'

export const DEFAULT_PQC_ALGORITHM: PQCAlgorithm = 'ML-DSA-65'

// FIPS 204 parameter sets; sizes are in bytes.
export const PQC_ALGORITHMS: {
  value: PQCAlgorithm
  level: string
  publicKeySize: number
  signatureSize: number
  description: string
}[] = [
  {
    value: 'ML-DSA-44',
    level: 'NIST mức 2',
    publicKeySize: 1312,
    signatureSize: 2420,
    description: 'Khóa và chữ ký nhỏ nhất, ký nhanh nhất. Phù hợp khi ưu tiên hiệu năng.'
  },
  {
    value: 'ML-DSA-65',
    level: 'NIST mức 3',
    publicKeySize: 1952,
    signatureSize: 3309,
    description: 'Cân bằng giữa độ an toàn và kích thước. Khuyến nghị cho văn bằng số.'
  },
  {
    value: 'ML-DSA-87',
    level: 'NIST mức 5',
    publicKeySize: 2592,
    signatureSize: 4627,
    description: 'An toàn cao nhất, chữ ký lớn nhất. Phù hợp khi cần bảo mật dài hạn rất cao.'
  }
]

export interface PQCKeyRecord {
  id: string
  university_id: string
  name: string
  algorithm: PQCAlgorithm
  status: PQCKeyStatus
  public_key_fingerprint: string
  created_at: string
  activated_at?: string
  retired_at?: string
  revoked_at?: string
  compromise_effective_at?: string
  revocation_reason?: string
}

export interface PQCVerificationResult {
  valid: boolean
  signature_valid: boolean
  cryptographically_valid: boolean
  key_valid_at_signing: boolean
  file_integrity_checked: boolean
  file_integrity_valid: boolean
  stored_file_hash?: string
  computed_file_hash?: string
  assurance_level: 'complete' | 'signature_only' | 'invalid'
  current_key_status: PQCKeyStatus | 'unknown'
  algorithm: string
  warning?: string
  error?: string
  blockchain: {
    status: 'verified' | 'not_anchored' | 'not_checked' | 'unavailable' | 'inconsistent' | 'mismatch' | string
    checked: boolean
    valid: boolean
    batch_id?: string
    transaction_id?: string
    merkle_root?: string
    error?: string
  }
}

export const getPQCKeys = async (): Promise<PQCKeyRecord[]> => {
  const response = await apiService('GET', 'pqc/keys')
  return Array.isArray(response.data) ? response.data : []
}

export const createPQCKey = async (name: string, algorithm: PQCAlgorithm = DEFAULT_PQC_ALGORITHM) => {
  return apiService('POST', 'pqc/keys', { name, algorithm })
}

export const activatePQCKey = async (id: string) => {
  return apiService('POST', `pqc/keys/${id}/activate`)
}

export const revokePQCKey = async (id: string, reason: string, compromiseEffectiveAt?: string) => {
  return apiService('POST', `pqc/keys/${id}/revoke`, {
    reason,
    compromise_effective_at: compromiseEffectiveAt || undefined
  })
}

export const signPQCEDiploma = async (id: string) => {
  return apiService('POST', `pqc/ediplomas/${id}/sign`)
}

export const verifyPQCEDiploma = async (id: string): Promise<PQCVerificationResult> => {
  const response = await apiService('GET', `pqc/ediplomas/${id}/verify`, undefined, false)
  return response.data
}
