import { ref, reactive, computed, watch, type ComputedRef, type Ref } from 'vue'
import type { ValidationRule, FieldError } from '@/utils/validation'

interface ValidationSchema {
  [field: string]: ValidationRule[]
}

interface FormValidationReturn {
  errors: Record<string, FieldError>
  touched: Record<string, boolean>
  isValid: ComputedRef<boolean>
  isDirty: ComputedRef<boolean>
  validateField: (field: string, value: any) => string | null
  validateAll: (data: Record<string, any>) => boolean
  touchField: (field: string) => void
  resetForm: () => void
  getFieldError: (field: string) => string | null
}

interface DebounceState {
  timers: Map<number, ReturnType<typeof setTimeout>>
  nextTimerId: number
}

function createDebounce(delay: number = 500): {
  debounce: (fn: () => void) => void
  clearAll: () => void
} {
  const state: DebounceState = {
    timers: new Map(),
    nextTimerId: 0,
  }

  return {
    debounce(fn: () => void) {
      const id = state.nextTimerId++
      state.timers.set(id, setTimeout(() => {
        state.timers.delete(id)
        fn()
      }, delay))
    },
    clearAll() {
      state.timers.forEach(timer => clearTimeout(timer))
      state.timers.clear()
    },
  }
}

export function useFormValidation(
  data: Record<string, any>,
  schema: ValidationSchema,
  options?: { debounceDelay?: number; validateOnBlurOnly?: boolean }
): FormValidationReturn {
  const { debounceDelay = 500, validateOnBlurOnly = false } = options || {}
  const { debounce, clearAll } = createDebounce(debounceDelay)

  const touched = reactive<Record<string, boolean>>({})
  const fieldErrors = reactive<Record<string, FieldError>>({})

  // 初始化所有字段为未触摸
  Object.keys(schema).forEach((field) => {
    touched[field] = false
    fieldErrors[field] = { field, message: '', touched: false }
  })

  // 验证单个字段
  const validateField = (field: string, value: any): string | null => {
    const rules = schema[field]
    if (!rules || rules.length === 0) return null

    for (const rule of rules) {
      const error = rule.validate(value)
      if (error) {
        return error
      }
    }

    return null
  }

  // 验证所有字段
  const validateAll = (formData: Record<string, any>): boolean => {
    let allValid = true

    // 标记所有字段为已触摸
    Object.keys(schema).forEach((field) => {
      touched[field] = true
      fieldErrors[field].touched = true

      const value = formData[field]
      const error = validateField(field, value)

      if (error) {
        fieldErrors[field] = { field, message: error, touched: true }
        allValid = false
      } else {
        fieldErrors[field] = { field, message: '', touched: true }
      }
    })

    return allValid
  }

  // 触摸字段（标记为已交互）
  const touchField = (field: string) => {
    touched[field] = true
    fieldErrors[field].touched = true

    clearAll()

    // 立即验证该字段
    const value = data[field]
    const error = validateField(field, value)

    if (error) {
      fieldErrors[field] = { field, message: error, touched: true }
    } else {
      fieldErrors[field] = { field, message: '', touched: true }
    }
  }

  // 获取字段错误消息（支持延迟显示）
  const getFieldError = (field: string): string | null => {
    const err = fieldErrors[field]
    if (!err || !err.touched || !err.message) return null
    return err.message
  }

  // 重置表单状态
  const resetForm = () => {
    clearAll()
    Object.keys(touched).forEach((field) => {
      touched[field] = false
      fieldErrors[field] = { field, message: '', touched: false }
    })
  }

  // 计算属性：整个表单是否有效
  const isValid = computed(() => {
    let valid = true
    for (const field of Object.keys(schema)) {
      const value = data[field]
      if (validateField(field, value) !== null) {
        valid = false
        break
      }
    }
    return valid
  })

  // 计算属性：是否有任何字段被修改过
  const isDirty = computed(() => {
    return Object.values(touched).some((t) => t)
  })

  // 自动监听数据变化，实时验证已触摸的字段（带防抖）
  if (!validateOnBlurOnly) {
    watch(
      () => ({ ...data }),
      (newData) => {
        debounce(() => {
          Object.keys(touched).forEach((field) => {
            if (touched[field]) {
              const error = validateField(field, newData[field])
              fieldErrors[field] = {
                field,
                message: error || '',
                touched: true,
              }
            }
          })
        })
      },
      { deep: true }
    )
  }

  return {
    errors: fieldErrors as unknown as Record<string, FieldError>,
    touched,
    isValid,
    isDirty,
    validateField,
    validateAll,
    touchField,
    resetForm,
    getFieldError,
  }
}
