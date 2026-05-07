<template>
  <div class="form-field" :class="{ 'has-error': showError, 'is-touched': touched && !error }">
    <label v-if="label" :for="inputId" class="field-label">
      {{ label }}
      <span v-if="required" class="text-error ml-1">*</span>
    </label>

    <div class="field-input-wrapper">
      <slot />

      <!-- 错误图标 -->
      <span v-if="showError" class="field-error-icon">⚠</span>

      <!-- 成功图标 -->
      <span v-else-if="!error && touched && value" class="field-success-icon">✓</span>
    </div>

    <!-- 错误消息（带延迟显示） -->
    <transition name="fade">
      <p v-if="showError" class="field-error-message">
        {{ error }}
      </p>
    </transition>

    <!-- 帮助文本 -->
    <p v-if="help && !showError" class="field-help-text">
      {{ help }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  label?: string
  error?: string | null
  touched?: boolean
  required?: boolean
  help?: string
  inputId?: string
  value?: any
  showErrorImmediately?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  error: null,
  touched: false,
  required: false,
  inputId: () => `field-${Math.random().toString(36).substr(2, 9)}`,
  showErrorImmediately: false,
})

// 控制是否显示错误（默认在失焦后显示，除非设置立即显示）
const showError = computed(() => {
  if (!props.error || !props.touched) return false

  // 如果设置了立即显示，或者值不为空且已触摸过，则显示错误
  return props.showErrorImmediately || (props.value !== '' && props.value !== null && props.value !== undefined)
})
</script>

<style scoped>
.form-field {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}

.field-label {
  font-size: 0.875rem;
  font-weight: 500;
  color: #a0aec0;
  display: block;
  margin-bottom: 0.25rem;
}

.field-input-wrapper {
  position: relative;
}

.field-input-wrapper :deep(input),
.field-input-wrapper :deep(select),
.field-input-wrapper :deep(textarea) {
  width: 100%;
  padding-right: 2.5rem; /* 为图标留出空间 */
}

.has-error .field-label {
  color: #f87171;
}

.has-error :deep(.input-field) {
  border-color: #ef4444;
  background-color: rgba(239, 68, 68, 0.05);
}

.has-error :deep(.input-field:focus) {
  border-color: #ef4444;
  box-shadow: 0 0 0 2px rgba(239, 68, 68, 0.2);
}

.is-touched:not(.has-error) :deep(.input-field) {
  border-color: #10b981;
}

.field-error-icon {
  position: absolute;
  right: 0.75rem;
  top: 50%;
  transform: translateY(-50%);
  color: #f87171;
  font-size: 1rem;
  pointer-events: none;
}

.field-success-icon {
  position: absolute;
  right: 0.75rem;
  top: 50%;
  transform: translateY(-50%);
  color: #10b981;
  font-size: 1rem;
  pointer-events: none;
}

.field-error-message {
  font-size: 0.75rem;
  color: #f87171;
  margin-top: 0.25rem;
  animation: fadeIn 0.2s ease-in;
}

.field-help-text {
  font-size: 0.75rem;
  color: #718096;
  margin-top: 0.125rem;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
