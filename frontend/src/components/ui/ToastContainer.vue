<script setup lang="ts">
import { globalDialog } from '@/composables/useDialog'
import Toast from './Toast.vue'

const { toasts, removeToast } = globalDialog
</script>

<template>
  <Teleport to="body">
    <div class="toast toast-end toast-top z-50">
      <TransitionGroup name="toast">
        <Toast
          v-for="t in toasts"
          :key="t.id"
          :id="String(t.id)"
          :message="t.message"
          :type="t.type"
          @close="removeToast(Number($event))"
        />
      </TransitionGroup>
    </div>
  </Teleport>
</template>

<style scoped>
.toast-enter-active,
.toast-leave-active {
  transition: all 0.3s ease;
}

.toast-enter-from {
  opacity: 0;
  transform: translateX(100%);
}

.toast-leave-to {
  opacity: 0;
  transform: translateX(100%);
}

.toast-move {
  transition: transform 0.3s ease;
}
</style>
