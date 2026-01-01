<script setup lang="ts">
import { ref, computed } from 'vue'
import moment from 'moment-timezone'
import type { Moment } from 'moment-timezone'
import linkifyStr from 'linkify-string'
import {
  User,
  AtSign,
  HelpCircle,
  Link,
  MapPin,
  Clock,
  ArrowUp,
  ArrowDown,
  X,
  Pin,
  Mail,
  Handshake,
  Frown,
  Circle,
  School,
  Undo2,
  Check,
} from 'lucide-vue-next'
import type { Queue, QueueEntry, RemovedQueueEntry } from '@/types'
import { useUserStore } from '@/stores/user'
import { useQueueStore } from '@/stores/queue'
import { globalDialog } from '@/composables/useDialog'
import ErrorDialog from '@/utils/ErrorDialog'
import { escapeHTML } from '@/utils/sanitization'
import * as PromptHandler from '@/utils/promptHandler'

const props = defineProps<{
  entry: QueueEntry | RemovedQueueEntry
  stack: boolean
  queue: Queue
  admin: boolean
  time: Moment
}>()

const userStore = useUserStore()
const queueStore = useQueueStore()

const removeRequestRunning = ref(false)
const helpingRequestRunning = ref(false)
const pinEntryRequestRunning = ref(false)
const notHelpedRequestRunning = ref(false)

const anonymous = computed(() => {
  return !(
    props.admin ||
    (userStore.userInfo?.email && props.entry.email === userStore.userInfo.email)
  )
})

const name = computed(() => {
  return anonymous.value ? 'Anonymous Student' : props.entry.name
})

const location = computed(() => {
  return linkifyStr(props.entry.location || '', {
    defaultProtocol: 'https',
  })
})

// For stack entries, use removed_at; for queue entries, use id_timestamp
// This matches the original: QueueEntry uses timestamp, RemovedQueueEntry uses removedAt
const entryMoment = computed(() => {
  if (props.stack && 'removed_at' in props.entry) {
    return moment(props.entry.removed_at).local()
  }
  return moment(props.entry.id_timestamp).local()
})

const humanizedTimestamp = computed(() => {
  // HACK: fix time update lag issues in the beginning
  // by saying the time is 5 seconds ahead of what it really is.
  // Since we only display "a few seconds ago" this shouldn't have
  // any noticeable impact.
  return entryMoment.value.from(props.time.clone().add(5, 'second'))
})

const tooltipTimestamp = computed(() => {
  return entryMoment.value.format('YYYY-MM-DD h:mm:ss a')
})

const isOnline = computed(() => {
  return props.entry.email ? queueStore.isUserOnline(props.entry.email) : false
})

const isBeingHelped = computed(() => {
  return 'helping' in props.entry && !!props.entry.helping
})

const beingHelpedBy = computed(() => {
  return 'helping' in props.entry ? props.entry.helping?.trim() || '' : ''
})

const config = computed(() => queueStore.config as Record<string, unknown> | null)

const promptResponses = computed(() => {
  return PromptHandler.descriptionToResponses(props.entry.description)
})

const hasCustomPrompts = computed(() => {
  return promptResponses.value.length > 0
})

async function removeEntry() {
  removeRequestRunning.value = true
  try {
    const res = await fetch(
      `/api/queues/${props.queue.id}/entries/${props.entry.id}`,
      { method: 'DELETE' }
    )
    if (res.status !== 204) {
      return ErrorDialog(res)
    }
  } finally {
    removeRequestRunning.value = false
  }
}

async function pinEntry() {
  pinEntryRequestRunning.value = true
  try {
    const res = await fetch(
      `/api/queues/${props.queue.id}/entries/${props.entry.id}/pin`,
      { method: 'POST' }
    )
    if (res.status !== 204) {
      return ErrorDialog(res)
    }
    globalDialog.toast(`Pinned ${escapeHTML(props.entry.email || '')}!`, 'success')
  } finally {
    pinEntryRequestRunning.value = false
  }
}

async function setHelping(helping: boolean) {
  helpingRequestRunning.value = true
  try {
    const res = await fetch(
      `/api/queues/${props.queue.id}/entries/${props.entry.id}/helping?helping=${helping}`,
      { method: 'PUT' }
    )
    if (res.status !== 204) {
      return ErrorDialog(res)
    }
  } finally {
    helpingRequestRunning.value = false
  }
}

async function setNotHelped() {
  notHelpedRequestRunning.value = true
  try {
    const res = await fetch(
      `/api/queues/${props.queue.id}/entries/${props.entry.id}/helped`,
      { method: 'DELETE' }
    )
    if (res.status !== 204) {
      return ErrorDialog(res)
    }
  } finally {
    notHelpedRequestRunning.value = false
  }
}

async function messageUser() {
  globalDialog.prompt({
    title: 'Send Message',
    message: `Send message to ${escapeHTML(props.entry.email || '')}:`,
    confirmText: 'Send',
    onConfirm: async (message) => {
      const res = await fetch(`/api/queues/${props.queue.id}/messages`, {
        method: 'POST',
        body: JSON.stringify({
          receiver: props.entry.email,
          content: message,
        }),
      })
      if (res.status !== 201) {
        return ErrorDialog(res)
      }
      globalDialog.toast(`Sent message to ${escapeHTML(props.entry.email || '')}`, 'success')
    },
  })
}
</script>

<template>
  <div class="card bg-base-100 shadow">
    <div class="card-body p-4">
      <div class="flex justify-between">
        <div class="flex-1 space-y-1">
          <!-- Name -->
          <div class="flex items-center gap-2">
            <User class="w-4 h-4 text-base-content/60 shrink-0" />
            <strong class="break-words">{{ name }}</strong>
          </div>

          <template v-if="!anonymous">
            <!-- Email -->
            <div class="flex items-center gap-2">
              <AtSign class="w-4 h-4 text-base-content/60 shrink-0" />
              <span class="break-all">{{ entry.email }}</span>
            </div>

            <!-- Description (custom prompts) -->
            <template v-if="hasCustomPrompts">
              <div v-for="(response, i) in promptResponses" :key="i" class="flex items-start gap-2">
                <HelpCircle class="w-4 h-4 text-base-content/60 shrink-0 mt-0.5" />
                <span class="break-words">{{ response }}</span>
              </div>
            </template>
            <!-- Description (plain text) -->
            <div v-else class="flex items-start gap-2">
              <HelpCircle class="w-4 h-4 text-base-content/60 shrink-0 mt-0.5" />
              <span class="break-words">{{ entry.description }}</span>
            </div>

            <!-- Location -->
            <div v-if="entry.location && entry.location !== '(disabled)'" class="flex items-start gap-2">
              <component
                :is="config?.virtual ? Link : MapPin"
                class="w-4 h-4 text-base-content/60 shrink-0 mt-0.5"
              />
              <span class="break-all" v-html="location"></span>
            </div>
          </template>

          <!-- Timestamp -->
          <div class="flex items-center gap-2">
            <Clock class="w-4 h-4 text-base-content/60 shrink-0" />
            <span class="tooltip" :data-tip="tooltipTimestamp">
              {{ humanizedTimestamp }}
            </span>
          </div>

          <!-- Priority -->
          <div v-if="entry.priority !== 0" class="flex items-center gap-2">
            <component
              :is="entry.priority > 0 ? ArrowUp : ArrowDown"
              class="w-4 h-4 text-base-content/60 shrink-0"
            />
            <span>Priority: {{ (entry.priority > 0 ? '+' : '') + entry.priority }}</span>
          </div>

          <!-- Removed by (stack only) -->
          <div v-if="stack && 'removed_by' in entry" class="flex items-center gap-2">
            <X class="w-4 h-4 text-base-content/60 shrink-0" />
            <span>{{ entry.removed_by }}</span>
          </div>
        </div>

        <!-- Status indicators -->
        <div class="flex flex-col items-end gap-2">
          <!-- Online status (admin only) -->
          <div v-if="admin" class="tooltip tooltip-left" :data-tip="`Student is ${isOnline ? 'online' : 'offline'}`">
            <Circle
              class="w-4 h-4"
              :class="isOnline ? 'fill-success text-success' : 'fill-error text-error'"
            />
          </div>

          <!-- Pinned indicator -->
          <div v-if="entry.pinned" class="tooltip tooltip-left" data-tip="Pinned to top">
            <Pin class="w-10 h-10" />
          </div>

          <!-- Being helped indicator -->
          <div v-if="isBeingHelped" class="tooltip tooltip-left" :data-tip="`Being helped by ${beingHelpedBy}`">
            <School class="w-10 h-10" />
          </div>

          <!-- Not helped indicator (stack only) -->
          <div v-if="stack && !entry.helped" class="tooltip tooltip-left" data-tip="Student wasn't helped">
            <Frown class="w-10 h-10" />
          </div>
        </div>
      </div>

      <!-- Action buttons -->
      <div v-if="!anonymous" class="flex flex-wrap gap-2 mt-4">
        <template v-if="!stack">
          <!-- Help button (admin, not being helped) -->
          <button
            v-if="admin && !isBeingHelped"
            class="btn btn-success btn-sm gap-2"
            :class="{ 'loading': helpingRequestRunning }"
            @click="setHelping(true)"
          >
            <Handshake class="w-4 h-4" />
            Help
          </button>

          <!-- Done/Undo buttons (admin, being helped) -->
          <template v-else-if="admin && isBeingHelped">
            <button
              class="btn btn-success btn-sm gap-2"
              :class="{ 'loading': removeRequestRunning }"
              @click="removeEntry"
            >
              <Check class="w-4 h-4" />
              Done
            </button>
            <button
              class="btn btn-error btn-sm gap-2"
              :class="{ 'loading': helpingRequestRunning }"
              @click="setHelping(false)"
            >
              <Undo2 class="w-4 h-4" />
              Undo
            </button>
          </template>

          <!-- Cancel button (student, own entry, not being helped) -->
          <button
            v-else-if="!admin && !isBeingHelped"
            class="btn btn-error btn-sm gap-2"
            :class="{ 'loading': removeRequestRunning }"
            @click="removeEntry"
          >
            <X class="w-4 h-4" />
            Cancel
          </button>
        </template>

        <!-- Pin button (admin, not pinned) -->
        <button
          v-if="admin && !entry.pinned"
          class="btn btn-primary btn-sm gap-2"
          :class="{ 'loading': pinEntryRequestRunning }"
          @click="pinEntry"
        >
          <Pin class="w-4 h-4" />
          Pin
        </button>

        <!-- Not helped button (stack, was helped) -->
        <button
          v-if="admin && stack && entry.helped"
          class="btn btn-error btn-sm gap-2"
          :class="{ 'loading': notHelpedRequestRunning }"
          @click="setNotHelped"
        >
          <Frown class="w-4 h-4" />
          Not helped
        </button>

        <!-- Message button (admin) -->
        <button
          v-if="admin"
          class="btn btn-warning btn-sm gap-2"
          :disabled="!isOnline"
          @click="messageUser"
        >
          <Mail class="w-4 h-4" />
          Message
        </button>
      </div>
    </div>
  </div>
</template>
