<script setup lang="ts">
import { computed } from 'vue'
import type { Moment } from 'moment-timezone'
import {
  DoorClosed,
  SmilePlus,
  HeartCrack,
  GraduationCap,
  Calendar,
  Lock,
  LockOpen,
  Dices,
  Eraser,
  Megaphone,
  Download,
} from 'lucide-vue-next'
import type { Queue } from '@/types'
import { useQueueStore } from '@/stores/queue'
import { useUserStore } from '@/stores/user'
import { globalDialog } from '@/composables/useDialog'
import ErrorDialog from '@/utils/ErrorDialog'
import QueueEntryDisplay from './QueueEntry.vue'
import QueueSignup from './QueueSignup.vue'

const props = defineProps<{
  queue: Queue
  loaded: boolean
  admin: boolean
  time: Moment
}>()

const queueStore = useQueueStore()
const userStore = useUserStore()

const config = computed(() => queueStore.config as Record<string, unknown> | null)

const isOpen = computed(() => {
  if (config.value?.scheduled) {
    // TODO: Implement scheduled open check
    return queueStore.queueOpen
  }
  return queueStore.queueOpen
})

const scheduledOpen = computed(() => {
  // TODO: Implement proper schedule checking
  return queueStore.queueOpen
})

const closesAt = computed(() => {
  // TODO: Implement schedule-based close time
  return 'later'
})

const opensAt = computed(() => {
  if (!config.value?.scheduled) {
    return `will be ${queueStore.queueOpen ? 'closed' : 'opened'} manually by staff`
  }
  return 'according to schedule'
})

async function clearQueue() {
  const confirmed = await globalDialog.confirm({
    title: 'Clear Queue',
    message: `Are you sure you want to clear the queue? <b>There's no undo; please don't use this to pop individual students.</b>`,
    type: 'danger',
    hasIcon: true,
    confirmText: 'Clear',
    cancelText: 'Cancel',
  })

  if (confirmed) {
    const res = await fetch(`/api/queues/${props.queue.id}/entries`, {
      method: 'DELETE',
    })
    if (res.status !== 204) {
      return ErrorDialog(res)
    }
  }
}

async function randomizeQueue() {
  const confirmed = await globalDialog.confirm({
    title: 'Randomize Queue',
    message: `Are you sure you want to randomize the queue? This will place everybody currently on the queue in a random position. <b>There's no undo.</b>`,
    type: 'danger',
    hasIcon: true,
    confirmText: 'Randomize',
    cancelText: 'Cancel',
  })

  if (confirmed) {
    const res = await fetch(`/api/queues/${props.queue.id}/entries/randomize`, {
      method: 'POST',
    })
    if (res.status !== 204) {
      return ErrorDialog(res)
    }
  }
}

async function setOpen(open: boolean) {
  const res = await fetch(
    `/api/queues/${props.queue.id}/configuration/manual-open?open=${open}`,
    { method: 'PUT' }
  )
  if (res.status !== 204) {
    return ErrorDialog(res)
  }
}

function broadcast() {
  globalDialog.prompt({
    title: 'Broadcast Message',
    message: 'Broadcast message to all online users of queue:',
    confirmText: 'Send',
    onConfirm: async (message) => {
      const res = await fetch(`/api/queues/${props.queue.id}/messages`, {
        method: 'POST',
        body: JSON.stringify({
          receiver: '<broadcast>',
          content: message,
        }),
      })
      if (res.status !== 201) {
        return ErrorDialog(res)
      }
      globalDialog.toast('Broadcast sent!', 'success')
    },
  })
}

function editSchedule() {
  // TODO: Implement schedule editor
  globalDialog.toast('Schedule editor coming soon', 'info')
}

function downloadStackAsCSV() {
  // TODO: Implement CSV download
  globalDialog.toast('CSV download coming soon', 'info')
}
</script>

<template>
  <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
    <!-- Queue column -->
    <div>
      <h2 class="text-2xl font-bold mb-4">Queue</h2>

      <div v-if="loaded">
        <!-- Queue entries -->
        <TransitionGroup name="entries" tag="div" class="space-y-4">
          <div v-for="entry in queueStore.sortedEntries" :key="entry.id">
            <QueueEntryDisplay
              :entry="entry"
              :stack="false"
              :queue="queue"
              :admin="admin"
              :time="time"
            />
          </div>
        </TransitionGroup>

        <!-- Empty state -->
        <div v-if="queueStore.entries.length === 0" class="bg-primary text-primary-content rounded-lg p-8">
          <!-- Queue closed -->
          <template v-if="!isOpen">
            <DoorClosed class="w-24 h-24 mb-4" />
            <h3 class="text-2xl font-bold">The queue is closed.</h3>
            <p class="opacity-90">
              See you next time{{ userStore.loggedIn ? ', ' + userStore.userInfo?.first_name : '' }}!
            </p>
          </template>
          <!-- Admin empty state -->
          <template v-else-if="admin">
            <SmilePlus class="w-24 h-24 mb-4" />
            <h3 class="text-2xl font-bold">The queue is empty.</h3>
            <p class="opacity-90">
              Good job, {{ userStore.userInfo?.first_name }}!
            </p>
          </template>
          <!-- Student empty state -->
          <template v-else>
            <HeartCrack class="w-24 h-24 mb-4" />
            <h3 class="text-2xl font-bold">The queue is empty.</h3>
            <p class="opacity-90">
              We're lonely over here{{ userStore.loggedIn ? ', ' + userStore.userInfo?.first_name : '' }}!
            </p>
          </template>
        </div>
      </div>

      <!-- Loading skeleton -->
      <div v-else class="space-y-4">
        <div v-for="i in 5" :key="i" class="card bg-base-100 shadow">
          <div class="card-body p-4">
            <div class="skeleton h-3 w-full mb-2"></div>
            <div class="skeleton h-3 w-full"></div>
          </div>
        </div>
      </div>
    </div>

    <!-- Sign up column -->
    <div>
      <!-- Queue stats -->
      <div v-if="loaded" class="flex items-center gap-4 mb-4 flex-wrap">
        <div class="flex items-center gap-2">
          <GraduationCap class="w-5 h-5" />
          <strong>{{ queueStore.entryCount }}</strong>
        </div>
        <p v-if="scheduledOpen" class="text-sm">
          The queue is open until {{ closesAt }}.
        </p>
        <p v-else class="text-sm">
          The queue {{ opensAt }}.
        </p>
      </div>
      <div v-else class="mb-4">
        <div class="skeleton h-4 w-48"></div>
      </div>

      <!-- Admin buttons -->
      <div v-if="admin" class="flex flex-wrap gap-2 mb-6 max-w-md">
        <button
          v-if="config?.scheduled"
          class="btn btn-primary gap-2"
          @click="editSchedule"
        >
          <Calendar class="w-4 h-4" />
          Edit Schedule
        </button>
        <button
          v-else-if="queueStore.queueOpen"
          class="btn btn-warning gap-2"
          @click="setOpen(false)"
        >
          <Lock class="w-4 h-4" />
          Close Queue
        </button>
        <button
          v-else
          class="btn btn-success gap-2"
          @click="setOpen(true)"
        >
          <LockOpen class="w-4 h-4" />
          Open Queue
        </button>
        <button class="btn btn-neutral gap-2" @click="randomizeQueue">
          <Dices class="w-4 h-4" />
          Randomize Queue
        </button>
        <button class="btn btn-error gap-2" @click="clearQueue">
          <Eraser class="w-4 h-4" />
          Clear Queue
        </button>
        <button class="btn bg-base-300 hover:bg-base-200 gap-2" @click="broadcast">
          <Megaphone class="w-4 h-4" />
          Broadcast to Queue
        </button>
      </div>

      <!-- Sign up form -->
      <div class="mb-6">
        <h2 class="text-2xl font-bold mb-4">Sign Up</h2>
        <QueueSignup :queue="queue" :time="time" />
      </div>

      <!-- Stack (admin only) -->
      <div v-if="admin && queueStore.stack.length > 0">
        <div class="flex items-center justify-between mb-4">
          <h2 class="text-2xl font-bold">Stack</h2>
          <button class="btn btn-primary btn-sm gap-2" @click="downloadStackAsCSV">
            <Download class="w-4 h-4" />
            Download
          </button>
        </div>
        <TransitionGroup name="entries" tag="div" class="space-y-4">
          <div v-for="entry in queueStore.stack" :key="entry.id">
            <QueueEntryDisplay
              :entry="entry"
              :stack="true"
              :queue="queue"
              :admin="admin"
              :time="time"
            />
          </div>
        </TransitionGroup>
      </div>
    </div>
  </div>
</template>

<style scoped>
.entries-enter-active,
.entries-leave-active {
  transition: all 0.5s ease;
}

.entries-enter-from {
  opacity: 0;
  transform: translateY(30px);
}

.entries-leave-to {
  opacity: 0;
  transform: translateY(-30px);
}

.entries-leave-active {
  position: absolute;
  width: 100%;
}

.entries-move {
  transition: transform 0.5s ease;
}
</style>
