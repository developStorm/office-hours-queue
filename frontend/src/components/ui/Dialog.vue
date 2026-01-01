<script setup lang="ts">
import { computed } from 'vue'
import { AlertCircle, AlertTriangle, Info, CheckCircle } from 'lucide-vue-next'
import { globalDialog } from '@/composables/useDialog'

const {
  dialogVisible,
  dialogOptions,
  closeDialog,
  promptVisible,
  promptOptions,
  promptValue,
  closePrompt,
} = globalDialog

const hasCancel = computed(() => !!dialogOptions.value?.cancelText)

const typeClass = computed(() => {
  const type = dialogOptions.value?.type
  if (type === 'danger') return 'text-error'
  if (type === 'warning') return 'text-warning'
  if (type === 'success') return 'text-success'
  return 'text-info'
})

const IconComponent = computed(() => {
  const type = dialogOptions.value?.type
  if (type === 'danger') return AlertCircle
  if (type === 'warning') return AlertTriangle
  if (type === 'success') return CheckCircle
  return Info
})

const confirmButtonClass = computed(() => {
  const type = dialogOptions.value?.type
  if (type === 'danger') return 'btn-error'
  if (type === 'warning') return 'btn-warning'
  if (type === 'success') return 'btn-success'
  return 'btn-primary'
})
</script>

<template>
  <Teleport to="body">
    <dialog
      class="modal"
      :class="{ 'modal-open': dialogVisible }"
      @click.self="hasCancel && closeDialog(false)"
    >
      <div class="modal-box" v-if="dialogOptions">
        <h3 class="font-bold text-lg flex items-center gap-2">
          <component
            v-if="dialogOptions.hasIcon"
            :is="IconComponent"
            :class="['w-6 h-6', typeClass]"
          />
          {{ dialogOptions.title }}
        </h3>
        <p class="py-4" v-html="dialogOptions.message"></p>
        <div class="modal-action">
          <button
            v-if="hasCancel"
            class="btn"
            @click="closeDialog(false)"
          >
            {{ dialogOptions.cancelText || 'Cancel' }}
          </button>
          <button
            :class="['btn', confirmButtonClass]"
            @click="closeDialog(true)"
          >
            {{ dialogOptions.confirmText || 'OK' }}
          </button>
        </div>
      </div>
    </dialog>

    <!-- Prompt Dialog -->
    <dialog
      class="modal"
      :class="{ 'modal-open': promptVisible }"
      @click.self="closePrompt(false)"
    >
      <div class="modal-box" v-if="promptOptions">
        <h3 class="font-bold text-lg">{{ promptOptions.title }}</h3>
        <p class="py-2">{{ promptOptions.message }}</p>
        <input
          v-model="promptValue"
          type="text"
          class="input input-bordered w-full mt-2"
          :placeholder="promptOptions.placeholder"
          @keydown.enter="closePrompt(true)"
        />
        <div class="modal-action">
          <button class="btn" @click="closePrompt(false)">
            {{ promptOptions.cancelText || 'Cancel' }}
          </button>
          <button class="btn btn-primary" @click="closePrompt(true)">
            {{ promptOptions.confirmText || 'OK' }}
          </button>
        </div>
      </div>
    </dialog>
  </Teleport>
</template>
