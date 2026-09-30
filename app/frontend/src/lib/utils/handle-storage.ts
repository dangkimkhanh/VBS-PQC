export const saveDataStorage = (key: string, data: any, type: 'local' | 'session' = 'local') => {
  if (typeof window === 'undefined') return

  const storage = type === 'session' ? window.sessionStorage : window.localStorage

  if (data === null || data === undefined) {
    console.log('No data to save into storage')
    return
  }

  // Chỉ update các field mới, giữ nguyên field cũ chưa được truyền vào
  if (typeof data === 'object' && !Array.isArray(data)) {
    const currentRaw = storage.getItem(key)
    let currentData: Record<string, any> = {}

    if (currentRaw) {
      try {
        const parsed = JSON.parse(currentRaw)
        if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
          currentData = parsed
        }
      } catch {
        currentData = {}
      }
    }

    const mergedData = { ...currentData, ...data }
    storage.setItem(key, JSON.stringify(mergedData))
    return
  }

  if (Array.isArray(data)) {
    storage.setItem(key, JSON.stringify(data))
    return
  }

  storage.setItem(key, String(data))
}

export const getDataStorage = (key: string, type: 'local' | 'session' = 'local') => {
  if (typeof window === 'undefined') return null

  const storage = type === 'session' ? window.sessionStorage : window.localStorage

  const dataStorage = storage.getItem(key)
  if (dataStorage === null) {
    return null
  } else {
    try {
      return JSON.parse(dataStorage)
    } catch {
      return dataStorage
    }
  }
}

export const removeDataStorage = (key: string, type: 'local' | 'session' = 'local') => {
  if (typeof window === 'undefined') return

  const storage = type === 'session' ? window.sessionStorage : window.localStorage
  storage.removeItem(key)
}

export const getSignDegreeConfig = (): {
  signService: string
  pdfSignLocation: string
  verifyService: string
  signingMethod: 'pqc' | 'usb'
} => {
  if (typeof window === 'undefined') {
    return {
      signService: '',
      pdfSignLocation: '',
      verifyService: '',
      signingMethod: 'pqc'
    }
  }

  return {
    signingMethod: 'pqc',
    ...(getDataStorage('setting') ?? {
      signService: '',
      pdfSignLocation: '',
      verifyService: ''
    })
  }
}
