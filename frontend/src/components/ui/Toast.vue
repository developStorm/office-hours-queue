<script setup lang="ts">
import { computed } from 'vue'
import { X, CheckCircle, AlertCircle, AlertTriangle, Info } from 'lucide-vue-next'

export interface ToastProps {
  id: string
  message: string
  type?: 'success' | 'error' | 'warning' | 'info'
  duration?: number
}

const props = withDefaults(defineProps<ToastProps>(), {
  type: 'info',
  duration: 5000,
})

const emit = defineEmits<{
  close: [id: string]
}>()

const alertClass = computed(() => {
  const classes: Record<string, string> = {
    success: 'alert-success',
    error: 'alert-error',
    warning: 'alert-warning',
    info: 'alert-info',
  }
  return classes[props.type] || 'alert-info'
})

const IconComponent = computed(() => {
  const icons = {
    success: CheckCircle,
    error: AlertCircle,
    warning: AlertTriangle,
    info: Info,
  }
  return icons[props.type] || Info
})
</script>

<template>
  <div :class="['alert', alertClass, 'shadow-lg']">
    <component :is="IconComponent" class="w-5 h-5" />
    <span>{{ message }}</span>
    <button class="btn btn-ghost btn-xs" @click="emit('close', id)">
      <X class="w-4 h-4" />
    </button>
  </div>
</template>
