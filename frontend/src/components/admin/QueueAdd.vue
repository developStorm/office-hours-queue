<script setup lang="ts">
import { ref } from 'vue'
import { X } from 'lucide-vue-next'
import { globalDialog } from '@/composables/useDialog'

const emit = defineEmits<{
  close: []
  saved: [name: string, location: string, type: string]
}>()

const name = ref('')
const location = ref('')
const type = ref('')

function saveQueue() {
  if (type.value === '') {
    globalDialog.alert({
      title: 'Error',
      message: 'Please select a queue type.',
      type: 'danger',
    })
    return
  }
  emit('saved', name.value, location.value, type.value)
}
</script>

<template>
  <div class="modal modal-open">
    <div class="modal-box">
      <div class="flex justify-between items-center mb-4">
        <h3 class="font-bold text-lg">Add Queue</h3>
        <button class="btn btn-ghost btn-sm btn-circle" @click="emit('close')">
          <X class="w-4 h-4" />
        </button>
      </div>

      <div class="space-y-4">
        <div class="form-control">
          <label class="label">
            <span class="label-text">Name</span>
          </label>
          <input
            v-model="name"
            type="text"
            class="input input-bordered w-full"
          />
        </div>

        <div class="form-control">
          <label class="label">
            <span class="label-text">Location</span>
          </label>
          <input
            v-model="location"
            type="text"
            class="input input-bordered w-full"
          />
        </div>

        <div class="form-control">
          <label class="label">
            <span class="label-text">Type (cannot be changed later!)</span>
          </label>
          <select v-model="type" class="select select-bordered w-full">
            <option value="" disabled>Select type...</option>
            <option value="ordered">ordered</option>
            <option value="appointments">appointments</option>
          </select>
        </div>
      </div>

      <div class="modal-action">
        <button class="btn" @click="emit('close')">Close</button>
        <button class="btn btn-success" @click="saveQueue">Save</button>
      </div>
    </div>
    <div class="modal-backdrop" @click="emit('close')"></div>
  </div>
</template>
