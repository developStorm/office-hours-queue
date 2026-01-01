<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { X } from 'lucide-vue-next'
import { globalDialog } from '@/composables/useDialog'
import { escapeHTML } from '@/utils/sanitization'

const props = defineProps<{
  defaultShortName: string
  defaultFullName: string
  defaultAdmins: string[]
}>()

const emit = defineEmits<{
  close: []
  saved: [shortName: string, fullName: string, admins: string[]]
}>()

const shortName = ref('')
const fullName = ref('')
const adminsText = ref('')

onMounted(() => {
  shortName.value = props.defaultShortName
  fullName.value = props.defaultFullName
  adminsText.value = JSON.stringify(props.defaultAdmins, null, 4)
})

function saveCourse() {
  try {
    const admins: string[] = JSON.parse(adminsText.value)
    if (!Array.isArray(admins) || admins.some((a) => typeof a !== 'string')) {
      globalDialog.alert({
        title: 'Error',
        message: 'Admins input is not array of strings',
        type: 'danger',
      })
      return
    }

    const allAdmins = new Set<string>()
    for (const a of admins) {
      if (allAdmins.has(a)) {
        globalDialog.alert({
          title: 'Error',
          message: `User ${escapeHTML(a)} appears in the admins array more than once.`,
          type: 'danger',
        })
        return
      }
      allAdmins.add(a)
    }

    emit('saved', shortName.value, fullName.value, admins)
  } catch {
    globalDialog.alert({
      title: 'Error',
      message: 'Admins input is not valid JSON.',
      type: 'danger',
    })
  }
}
</script>

<template>
  <div class="modal modal-open">
    <div class="modal-box max-w-2xl">
      <div class="flex justify-between items-center mb-4">
        <h3 class="font-bold text-lg">Course Info</h3>
        <button class="btn btn-ghost btn-sm btn-circle" @click="emit('close')">
          <X class="w-4 h-4" />
        </button>
      </div>

      <div class="space-y-4">
        <div class="form-control">
          <label class="label">
            <span class="label-text">Short Name</span>
          </label>
          <input
            v-model="shortName"
            type="text"
            class="input input-bordered w-full"
          />
        </div>

        <div class="form-control">
          <label class="label">
            <span class="label-text">Full Name</span>
          </label>
          <input
            v-model="fullName"
            type="text"
            class="input input-bordered w-full"
          />
        </div>

        <div class="form-control">
          <label class="label">
            <span class="label-text">Course Admins (JSON array of emails)</span>
          </label>
          <textarea
            v-model="adminsText"
            class="textarea textarea-bordered w-full h-32"
          ></textarea>
        </div>
      </div>

      <div class="modal-action">
        <button class="btn" @click="emit('close')">Close</button>
        <button class="btn btn-success" @click="saveCourse">Save</button>
      </div>
    </div>
    <div class="modal-backdrop" @click="emit('close')"></div>
  </div>
</template>
