export function escapeHtml(str: string): string {
  if (!str) return ''
  
  const htmlEscapeMap: Record<string, string> = {
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;',
    '/': '&#x2F;',
    '`': '&#x60;',
    '=': '&#x3D;',
  }
  
  return String(str).replace(/[&<>"'`=/]/g, (char) => htmlEscapeMap[char])
}

export function sanitizeInput(input: string): string {
  if (!input) return ''
  
  return input
    .replace(/<script\b[^<]*(?:(?!<\/script>)<[^<]*)*<\/script>/gi, '')
    .replace(/on\w+\s*=\s*"[^"]*"/gi, '')
    .replace(/on\w+\s*=\s*'[^']*'/gi, '')
    .replace(/javascript:/gi, '')
    .trim()
}

export function isSafeUrl(url: string): boolean {
  if (!url) return false
  
  const safeUrlPattern = /^https?:\/\/[^\s/$.?#].[^\s]*$/
  const dataUrlPattern = /^data:image\/(png|jpeg|gif|webp);base64,[a-zA-Z0-9+/=]+$/
  
  return safeUrlPattern.test(url) || dataUrlPattern.test(url)
}

export function validateApiKeyValue(value: string): { valid: boolean; sanitized: string; error?: string } {
  if (!value || value.length < 10) {
    return { valid: false, sanitized: '', error: 'API Key 至少需要 10 个字符' }
  }

  if (/[<>\"']/.test(value)) {
    return { valid: false, sanitized: '', error: 'API Key 包含非法字符' }
  }

  const sanitized = value.trim()
  
  return { valid: true, sanitized }
}

export function validateConfigPath(path: string): { valid: boolean; sanitized: string; error?: string } {
  if (!path) {
    return { valid: false, sanitized: '', error: '配置路径不能为空' }
  }

  const normalizedPath = path
    .replace(/\.\./g, '')
    .replace(/[<>:"|?*]/g, '')
    .trim()

  if (normalizedPath !== path) {
    return { 
      valid: false, 
      sanitized: normalizedPath, 
      error: '配置路径包含非法字符，已自动清理' 
    }
  }

  return { valid: true, sanitized: normalizedPath }
}

const xssPattern = /<script\b[^<]*(?:(?!<\/script>)<[^<]*)*<\/script>|javascript:|on\w+\s*=/gi

export function containsXss(content: string): boolean {
  return xssPattern.test(content)
}
