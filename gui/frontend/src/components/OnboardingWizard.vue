<template>
  <div v-if="showOnboarding" class="fixed inset-0 z-50 bg-bg-primary/95 backdrop-blur-sm flex items-center justify-center p-4">
    <div class="bg-bg-secondary rounded-2xl shadow-2xl max-w-2xl w-full max-h-[90vh] overflow-hidden border border-border">
      <!-- Header -->
      <div class="px-8 py-6 border-b border-border">
        <div class="flex items-center justify-between">
          <h2 class="text-2xl font-bold text-white">欢迎使用 OpenHijack</h2>
          <button 
            @click="skipOnboarding" 
            class="text-text-muted hover:text-white transition-colors text-sm"
            aria-label="跳过向导"
          >
            跳过 →
          </button>
        </div>
        
        <!-- Progress Steps -->
        <div class="mt-6 flex items-center gap-2">
          <div 
            v-for="(step, index) in steps" 
            :key="step.id"
            class="flex-1 flex items-center"
          >
            <div 
              class="flex items-center gap-2 flex-1 p-2 rounded-lg transition-all cursor-pointer"
              :class="{
                'bg-primary-light/20': currentStepIndex === index,
                'hover:bg-bg-tertiary': currentStepIndex !== index,
              }"
              @click="goToStep(index)"
            >
              <div 
                class="w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold transition-all"
                :class="{
                  'bg-primary-light text-white': currentStepIndex === index,
                  'bg-green-500 text-white': index < currentStepIndex,
                  'bg-bg-tertiary text-text-muted': index > currentStepIndex,
                }"
              >
                {{ index < currentStepIndex ? '✓' : index + 1 }}
              </div>
              <span 
                class="text-sm hidden sm:block"
                :class="{
                  'text-primary-light font-medium': currentStepIndex === index,
                  'text-green-400': index < currentStepIndex,
                  'text-text-muted': index > currentStepIndex,
                }"
              >
                {{ step.title }}
              </span>
            </div>
            
            <div 
              v-if="index < steps.length - 1" 
              class="w-8 h-0.5 mx-1 transition-colors"
              :class="index < currentStepIndex ? 'bg-green-500' : 'bg-border'"
            ></div>
          </div>
        </div>
      </div>

      <!-- Content -->
      <div class="p-8 overflow-y-auto max-h-[calc(90vh-200px)]">
        <transition name="fade" mode="out-in">
          <component 
            :is="currentStep.component" 
            v-model="wizardData"
            @next="nextStep"
            @prev="prevStep"
            @complete="completeOnboarding"
          />
        </transition>
      </div>

      <!-- Footer -->
      <div class="px-8 py-4 border-t border-border bg-bg-primary/50 flex items-center justify-between">
        <button
          v-if="currentStepIndex > 0"
          @click="prevStep"
          class="btn-outline"
        >
          ← 上一步
        </button>
        
        <div v-else></div>

        <div class="flex gap-3">
          <button
            v-if="currentStepIndex < steps.length - 1"
            @click="nextStep"
            :disabled="!canProceed"
            class="btn-primary"
            :class="{ 'opacity-50 cursor-not-allowed': !canProceed }"
          >
            下一步 →
          </button>
          
          <button
            v-else
            @click="completeOnboarding"
            :disabled="!canComplete"
            class="btn-primary bg-green-600 hover:bg-green-700"
            :class="{ 'opacity-50 cursor-not-allowed': !canComplete }"
          >
            🎉 开始使用
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, markRaw } from 'vue'
import WelcomeStep from './onboarding/WelcomeStep.vue'
import ProviderSelectStep from './onboarding/ProviderSelectStep.vue'
import ApiKeyConfigStep from './onboarding/ApiKeyConfigStep.vue'
import ConnectionTestStep from './onboarding/ConnectionTestStep.vue'
import CompleteStep from './onboarding/CompleteStep.vue'
import { useOnboardingStore } from '@/stores/onboarding'

const onboardingStore = useOnboardingStore()

interface Step {
  id: string
  title: string
  component: any
}

const steps: Step[] = [
  { id: 'welcome', title: '欢迎', component: markRaw(WelcomeStep) },
  { id: 'provider', title: '选择供应商', component: markRaw(ProviderSelectStep) },
  { id: 'config', title: '配置密钥', component: markRaw(ApiKeyConfigStep) },
  { id: 'test', title: '测试连接', component: markRaw(ConnectionTestStep) },
  { id: 'complete', title: '完成', component: markRaw(CompleteStep) },
]

const showOnboarding = computed(() => onboardingStore.showOnboarding)
const currentStepIndex = ref(0)
const wizardData = ref({
  provider: '',
  api_url: '',
  model_id: '',
  api_key: '',
  auth_key: '',
  mapped_model_id: '',
})

const currentStep = computed(() => steps[currentStepIndex.value])

const canProceed = computed(() => {
  switch (currentStep.value.id) {
    case 'welcome':
      return true
    case 'provider':
      return wizardData.value.provider !== ''
    case 'config':
      return (
        wizardData.value.api_key !== '' &&
        wizardData.value.api_url !== '' &&
        wizardData.value.auth_key !== ''
      )
    case 'test':
      return true // 允许跳过测试
    default:
      return false
  }
})

const canComplete = computed(() => {
  return (
    wizardData.value.provider !== '' &&
    wizardData.value.api_key !== ''
  )
})

function nextStep() {
  if (currentStepIndex.value < steps.length - 1 && canProceed.value) {
    currentStepIndex.value++
  }
}

function prevStep() {
  if (currentStepIndex.value > 0) {
    currentStepIndex.value--
  }
}

function goToStep(index: number) {
  if (index <= currentStepIndex.value || canProceed.value) {
    currentStepIndex.value = index
  }
}

async function completeOnboarding() {
  try {
    await onboardingStore.completeOnboarding(wizardData.value)
  } catch (error) {
    console.error('Failed to complete onboarding:', error)
  }
}

function skipOnboarding() {
  if (confirm('确定要跳过设置向导吗？您可以稍后在设置页面中配置。')) {
    onboardingStore.skipOnboarding()
  }
}
</script>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
