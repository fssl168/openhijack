export interface ValidationRule {
  name: string
  validate: (value: any) => string | null
  message?: string
}

export interface FieldError {
  field: string
  message: string
  touched: boolean
}

export interface FormState {
  isValid: boolean
  isDirty: boolean
  isSubmitting: boolean
  errors: Record<string, FieldError>
}

// 内置验证规则
export const required = (message?: string): ValidationRule => ({
  name: 'required',
  validate: (value: any) => {
    if (value === null || value === undefined || value === '') {
      return message || '此字段为必填项'
    }
    if (typeof value === 'string' && value.trim() === '') {
      return message || '此字段不能为空'
    }
    return null
  },
})

export const minLength = (min: number, message?: string): ValidationRule => ({
  name: 'minLength',
  validate: (value: any) => {
    if (typeof value !== 'string') return null
    if (value.length < min) {
      return message || `最少需要 ${min} 个字符`
    }
    return null
  },
})

export const maxLength = (max: number, message?: string): ValidationRule => ({
  name: 'maxLength',
  validate: (value: any) => {
    if (typeof value !== 'string') return null
    if (value.length > max) {
      return message || `最多允许 ${max} 个字符`
    }
    return null
  },
})

export const url = (message?: string): ValidationRule => ({
  name: 'url',
  validate: (value: any) => {
    if (!value) return null
    try {
      new URL(value)
      return null
    } catch {
      return message || '请输入有效的 URL 地址'
    }
  },
})

export const flexibleUrl = (message?: string): ValidationRule => ({
  name: 'flexibleUrl',
  validate: (value: any) => {
    if (!value) return null

    const urlStr = String(value).trim()

    if (urlStr.match(/^https?:\/\/.+/i)) {
      try {
        new URL(urlStr)
        return null
      } catch {
        return message || 'URL 格式不正确'
      }
    }

    if (urlStr.match(/^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)*$/)) {
      return null
    }

    if (urlStr.match(/^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$/)) {
      return null
    }

    return message || '请输入有效的域名或 URL（如 api.example.com 或 https://api.example.com）'
  },
})

export const port = (message?: string): ValidationRule => ({
  name: 'port',
  validate: (value: any) => {
    if (!value && value !== 0) return null
    const num = Number(value)
    if (isNaN(num) || num < 1 || num > 65535) {
      return message || '端口号范围: 1-65535'
    }
    if (num < 1024) {
      return message || '端口号 < 1024 需要 root 权限'
    }
    return null
  },
})

export const email = (message?: string): ValidationRule => ({
  name: 'email',
  validate: (value: any) => {
    if (!value) return null
    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
    if (!emailRegex.test(value)) {
      return message || '请输入有效的邮箱地址'
    }
    return null
  },
})

export const pattern = (regex: RegExp, message: string): ValidationRule => ({
  name: 'pattern',
  validate: (value: any) => {
    if (!value) return null
    if (!regex.test(value)) {
      return message
    }
    return null
  },
})

export const custom = (
  validator: (value: any) => string | null,
  name = 'custom'
): ValidationRule => ({
  name,
  validate: validator,
})
