import { ref } from 'vue'

// Toast state
interface Toast {
  id: number
  message: string
  type: 'info' | 'success' | 'warning' | 'error'
}

const toasts = ref<Toast[]>([])
let toastId = 0

// Dialog state
interface DialogOptions {
  title: string
  message: string
  type?: 'info' | 'success' | 'warning' | 'danger'
  confirmText?: string
  cancelText?: string
  hasIcon?: boolean
}

interface PromptOptions {
  title: string
  message: string
  confirmText?: string
  cancelText?: string
  placeholder?: string
  defaultValue?: string
  onConfirm?: (value: string) => void
}

const dialogVisible = ref(false)
const dialogOptions = ref<DialogOptions | null>(null)
let dialogResolve: ((confirmed: boolean) => void) | null = null

const promptVisible = ref(false)
const promptOptions = ref<PromptOptions | null>(null)
const promptValue = ref('')

export function useDialog() {
  // Toast methods
  function toast(
    message: string,
    type: Toast['type'] = 'info',
    duration = 5000
  ) {
    const id = toastId++
    toasts.value.push({ id, message, type })

    setTimeout(() => {
      toasts.value = toasts.value.filter((t) => t.id !== id)
    }, duration)
  }

  function removeToast(id: number) {
    toasts.value = toasts.value.filter((t) => t.id !== id)
  }

  // Dialog methods
  function alert(options: Omit<DialogOptions, 'cancelText'>): Promise<void> {
    return new Promise((resolve) => {
      dialogOptions.value = { ...options, cancelText: undefined }
      dialogVisible.value = true
      dialogResolve = () => resolve()
    })
  }

  function confirm(options: DialogOptions): Promise<boolean> {
    return new Promise((resolve) => {
      dialogOptions.value = options
      dialogVisible.value = true
      dialogResolve = resolve
    })
  }

  function closeDialog(confirmed: boolean) {
    dialogVisible.value = false
    dialogResolve?.(confirmed)
    dialogResolve = null
    dialogOptions.value = null
  }

  // Prompt methods
  function prompt(options: PromptOptions): void {
    promptOptions.value = options
    promptValue.value = options.defaultValue || ''
    promptVisible.value = true
  }

  function closePrompt(confirmed: boolean) {
    if (confirmed && promptOptions.value?.onConfirm) {
      promptOptions.value.onConfirm(promptValue.value)
    }
    promptVisible.value = false
    promptOptions.value = null
    promptValue.value = ''
  }

  return {
    // Toast
    toasts,
    toast,
    removeToast,

    // Dialog
    dialogVisible,
    dialogOptions,
    alert,
    confirm,
    closeDialog,

    // Prompt
    promptVisible,
    promptOptions,
    promptValue,
    prompt,
    closePrompt,
  }
}

// Singleton for global access
export const globalDialog = useDialog()
