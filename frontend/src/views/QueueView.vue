<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Settings, EthernetPort } from 'lucide-vue-next'
import moment from 'moment-timezone'
import { useAppStore } from '@/stores/app'
import { useUserStore } from '@/stores/user'
import { useQueueStore } from '@/stores/queue'
import { globalDialog } from '@/composables/useDialog'
import AnnouncementDisplay from '@/components/AnnouncementDisplay.vue'
import OrderedQueueDisplay from '@/components/ordered/OrderedQueue.vue'
import QueueManage from '@/components/admin/QueueManage.vue'
import ErrorDialog from '@/utils/ErrorDialog'

const props = defineProps<{
  studentView: boolean
}>()

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const userStore = useUserStore()
const queueStore = useQueueStore()

const found = ref(false)
const loaded = ref(false)
const time = ref(moment())
const lastMessage = ref<moment.Moment | undefined>()
const showManageModal = ref(false)
const manageModalProps = ref<{
  type: 'ordered' | 'appointments'
  defaultConfiguration: Record<string, unknown>
  defaultGroups: string[][]
}>({
  type: 'ordered',
  defaultConfiguration: {},
  defaultGroups: [],
})

let ws: WebSocket | null = null
let timeUpdater: number | undefined

const queueId = computed(() => route.params.qid as string)

const queue = computed(() => {
  return appStore.queues[queueId.value] || null
})

const admin = computed(() => {
  return !!(
    !props.studentView &&
    userStore.userInfo?.admin_courses?.includes(queue.value?.course?.id || '')
  )
})

function initWebSocket() {
  const id = queueId.value
  if (!id) return

  const url = new URL(
    import.meta.env.BASE_URL + `api/queues/${id}/ws`,
    window.location.href
  )
  url.protocol = url.protocol.replace('http', 'ws')
  ws = new WebSocket(url.href)

  // Register WS with store for PONG responses
  queueStore.setWebSocket(ws)

  ws.onopen = async () => {
    await queueStore.fetchQueueInfo(id)
    loaded.value = true
  }

  ws.onclose = (c) => {
    queueStore.setWebSocket(null)
    if (c.code !== 1005) {
      console.log('WebSocket disconnected:', c)
      globalDialog.toast('It looks like you got disconnected. Refreshing...', 'error')
      setTimeout(() => location.reload(), 2000)
    }
  }

  ws.onmessage = (e) => {
    lastMessage.value = moment()
    const msg = JSON.parse(e.data)
    queueStore.handleWebSocketMessage(msg.e, msg.d, queue.value?.course?.short_name, admin.value)
  }
}

async function openManageDialog() {
  if (!queue.value) return

  try {
    const [configRes, groupsRes] = await Promise.all([
      fetch(`/api/queues/${queue.value.id}/configuration`),
      fetch(`/api/queues/${queue.value.id}/groups`),
    ])

    const [configuration, groups] = await Promise.all([
      configRes.json(),
      groupsRes.json(),
    ])

    manageModalProps.value = {
      type: queue.value.type,
      defaultConfiguration: configuration,
      defaultGroups: groups || [],
    }
    showManageModal.value = true
  } catch (e) {
    console.error('Failed to load queue config:', e)
  }
}

async function handleConfigurationSaved(config: Record<string, unknown>, promptsInput: string) {
  if (!queue.value) return

  if (promptsInput === '') promptsInput = '[]'

  try {
    const prompts = JSON.parse(promptsInput)
    if (!Array.isArray(prompts) || !prompts.every((p) => typeof p === 'string')) {
      globalDialog.alert({
        title: 'Error',
        message: 'Custom prompts must be a valid JSON array of strings',
        type: 'danger',
      })
      return
    }
    config.prompts = prompts
  } catch {
    globalDialog.alert({
      title: 'Error',
      message: 'Custom prompts must be a valid JSON array of strings',
      type: 'danger',
    })
    return
  }

  const res = await fetch(`/api/queues/${queue.value.id}/configuration`, {
    method: 'PUT',
    body: JSON.stringify(config),
  })

  if (res.status !== 204) {
    return ErrorDialog(res)
  }

  globalDialog.toast('Queue settings saved!', 'success')
}

async function handleGroupsSaved(groups: string[][]) {
  if (!queue.value) return

  const res = await fetch(`/api/queues/${queue.value.id}/groups`, {
    method: 'PUT',
    body: JSON.stringify(groups),
  })

  if (res.status !== 204) {
    return ErrorDialog(res)
  }

  globalDialog.toast('Queue groups saved!', 'success')
}

async function handleAnnouncementAdded(content: string) {
  if (!queue.value) return

  const res = await fetch(`/api/queues/${queue.value.id}/announcements`, {
    method: 'POST',
    body: JSON.stringify({ content }),
  })

  if (res.status !== 201) {
    return ErrorDialog(res)
  }

  globalDialog.toast('Announcement added!', 'success')
}

onMounted(() => {
  // Check if queue exists
  if (!queue.value) {
    globalDialog.toast("I couldn't find that queue! Bringing you back home...", 'error')
    router.push('/')
    return
  }

  found.value = true

  // Collapse sidebar on queue page
  appStore.showCourses = false

  // Update time periodically
  timeUpdater = window.setInterval(() => {
    time.value = moment()
    // Reload if no message received in 12+ seconds
    if (lastMessage.value && time.value.diff(lastMessage.value, 'seconds') > 12) {
      location.reload()
    }
  }, 5000)

  // Set page title
  if (queue.value.course) {
    document.title = `${queue.value.course.short_name} Office Hours`
  }

  // Initialize WebSocket
  initWebSocket()
})

onUnmounted(() => {
  if (ws) {
    ws.close()
  }
  queueStore.setWebSocket(null)
  if (timeUpdater) {
    clearInterval(timeUpdater)
  }
})

// Watch for route changes
watch(queueId, () => {
  loaded.value = false
  if (ws) {
    ws.close()
  }
  initWebSocket()
})
</script>

<template>
  <div v-if="found" class="card bg-base-100 shadow-sm">
    <div class="card-body relative">
      <!-- Admin buttons -->
      <div v-if="admin" class="absolute top-4 right-4 flex items-center gap-2">
        <div class="tooltip tooltip-left" data-tip="Active connections">
          <span class="flex items-center gap-2 text-base-content/70">
            <EthernetPort class="w-4 h-4" />
            <span class="font-bold">{{ queueStore.websocketConnections }}</span>
          </span>
        </div>
        <button class="btn bg-base-300 hover:bg-base-200 gap-2" @click="openManageDialog">
          <Settings class="w-4 h-4" />
          <span>Manage Queue</span>
        </button>
      </div>

      <!-- Announcements -->
      <section v-if="queueStore.announcements.length" class="mb-8">
        <h2 class="text-2xl font-bold mb-4">Announcements</h2>
        <div v-for="announcement in queueStore.announcements" :key="announcement.id" class="mb-4">
          <AnnouncementDisplay
            :announcement="announcement"
            :queue-id="queueId"
            :admin="admin"
          />
        </div>
      </section>

      <!-- Queue content -->
      <section v-if="queue?.type === 'ordered'">
        <OrderedQueueDisplay
          :queue="queue"
          :loaded="loaded"
          :admin="admin"
          :time="time"
        />
      </section>
      <section v-else-if="queue?.type === 'appointments'">
        <div class="alert alert-info">
          Appointments queue view coming soon
        </div>
      </section>
    </div>
  </div>

  <!-- Queue Manage Modal -->
  <QueueManage
    v-if="showManageModal"
    v-bind="manageModalProps"
    @close="showManageModal = false"
    @announcement-added="handleAnnouncementAdded"
    @configuration-saved="handleConfigurationSaved"
    @groups-saved="handleGroupsSaved"
  />
</template>
