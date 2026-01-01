<script setup lang="ts">
import { computed } from 'vue'
import { X } from 'lucide-vue-next'
import linkifyStr from 'linkify-string'
import type { Announcement } from '@/types'
import { globalDialog } from '@/composables/useDialog'
import ErrorDialog from '@/utils/ErrorDialog'

const props = defineProps<{
  announcement: Announcement
  queueId: string
  admin: boolean
}>()

const linkifiedContent = computed(() => {
  return linkifyStr(props.announcement.content, {
    defaultProtocol: 'https',
  })
})

async function removeAnnouncement() {
  const confirmed = await globalDialog.confirm({
    title: 'Delete Announcement',
    message: `Are you sure you want to delete this announcement? <b>There's no undo; this will remove this announcement for everyone.</b>`,
    type: 'danger',
    hasIcon: true,
    confirmText: 'Delete',
    cancelText: 'Cancel',
  })

  if (confirmed) {
    const res = await fetch(
      `/api/queues/${props.queueId}/announcements/${props.announcement.id}`,
      { method: 'DELETE' }
    )
    if (res.status !== 204) {
      return ErrorDialog(res)
    }
  }
}
</script>

<template>
  <div class="alert alert-warning relative">
    <button
      v-if="admin"
      class="btn btn-ghost btn-xs btn-circle absolute top-2 right-2"
      @click="removeAnnouncement"
    >
      <X class="w-4 h-4" />
    </button>
    <p class="break-words" v-html="linkifiedContent"></p>
  </div>
</template>
