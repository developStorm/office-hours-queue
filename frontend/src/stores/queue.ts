import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import moment from 'moment-timezone'
import type { QueueEntry, Announcement, RemovedQueueEntry, QueueConfiguration } from '@/types'
import { globalDialog } from '@/composables/useDialog'
import { escapeHTML } from '@/utils/sanitization'
import { sendNotification } from '@/utils/notification'

export const useQueueStore = defineStore('queue', () => {
  // State
  const currentQueueId = ref<string | null>(null)
  const entries = ref<QueueEntry[]>([])
  const stack = ref<RemovedQueueEntry[]>([])
  const announcements = ref<Announcement[]>([])
  const online = ref<Set<string>>(new Set())
  const websocketConnections = ref(0)
  const queueOpen = ref(false)
  const config = ref<QueueConfiguration | null>(null)
  const schedule = ref<string | undefined>(undefined)

  // Track if current user is admin (set from QueueView)
  const isAdmin = ref(false)

  // Getters
  const sortedEntries = computed(() => {
    // Match original sorting logic from OrderedQueue.ts
    return [...entries.value].sort((a, b) => {
      // Pinned entries first
      if (a.pinned !== b.pinned) {
        return a.pinned ? -1 : 1
      }

      // Being helped entries second
      const aHelping = a.helping !== ''
      const bHelping = b.helping !== ''
      if (aHelping !== bHelping) {
        return aHelping ? -1 : 1
      }

      // Then by priority (higher priority first)
      if (a.priority !== b.priority) {
        return b.priority - a.priority
      }

      // Finally by ID (earlier first)
      return a.id < b.id ? -1 : a.id > b.id ? 1 : 0
    })
  })

  const entryCount = computed(() => entries.value.length)

  const isUserOnline = (email: string) => online.value.has(email)

  // Actions
  function setCurrentQueue(queueId: string) {
    currentQueueId.value = queueId
    entries.value = []
    stack.value = []
    announcements.value = []
    online.value = new Set()
  }

  function setAdmin(admin: boolean) {
    isAdmin.value = admin
  }

  // WebSocket message handlers - matching original OrderedQueue.ts

  function handleEntryCreate(data: Record<string, unknown>, adminView: boolean) {
    // Normalize the entry with defaults
    const entry = normalizeEntry(data)

    // Set online status from our tracking
    if (entry.email) {
      entry.online = online.value.has(entry.email)
    }

    const existingIndex = entries.value.findIndex(e => e.id === entry.id)
    if (existingIndex !== -1) {
      entries.value[existingIndex] = entry
      sortEntries()
      return
    }

    // Admin notifications - matching original
    if (adminView) {
      globalDialog.toast(`${escapeHTML(entry.email || '')} joined the queue!`, 'info', 2000)

      // Send browser notification if queue was empty
      if (entries.value.length === 0) {
        sendNotification(
          'A new student joined the queue!',
          `A wild ${entry.email} has appeared!`
        )
      }
    }

    entries.value.push(entry)
    sortEntries()
  }

  function handleEntryRemove(data: Record<string, unknown>) {
    const id = data.id as string
    const index = entries.value.findIndex(e => e.id === id)
    if (index !== -1) {
      entries.value.splice(index, 1)
    }

    // Add to stack if full entry data provided
    if (data.email) {
      // Normalize the entry with defaults
      const removedEntry = normalizeRemovedEntry(data)
      // Set online status
      removedEntry.online = online.value.has(removedEntry.email || '')
      stack.value.unshift(removedEntry)
      sortStack()
    }
  }

  function handleEntryUpdate(data: Record<string, unknown>) {
    const id = data.id as string

    // Check queue entries first
    const index = entries.value.findIndex(e => e.id === id)
    if (index !== -1) {
      // Normalize and update
      const entry = normalizeEntry(data)
      if (entry.email) {
        entry.online = online.value.has(entry.email)
      }
      entries.value[index] = entry
      sortEntries()
    } else {
      // Check stack entries - matching original
      const stackIndex = stack.value.findIndex(e => e.id === id)
      if (stackIndex !== -1) {
        const removedEntry = normalizeRemovedEntry(data)
        if (removedEntry.email) {
          removedEntry.online = online.value.has(removedEntry.email)
        }
        stack.value[stackIndex] = removedEntry
      }
    }
  }

  function handleStackRemove(data: { id: string }) {
    const index = stack.value.findIndex(e => e.id === data.id)
    if (index !== -1) {
      stack.value.splice(index, 1)
    }
  }

  function handleQueueOpen(data: boolean) {
    const nowOpen = data
    queueOpen.value = nowOpen

    // Show toast notification - matching original
    globalDialog.toast(
      `The queue is now ${nowOpen ? 'open!' : 'closed.'}`,
      nowOpen ? 'success' : 'error',
      10000
    )
  }

  function handleQueueClear(removedBy: string | null, adminView: boolean) {
    // Estimate what the stack will look like based on
    // the information received from the event - matching original
    const removed: RemovedQueueEntry[] = entries.value.map(e => ({
      ...e,
      pinned: false,
      removed_at: moment().format(),
      removed_by: removedBy || 'System',
      helped: true,
    }))

    entries.value = []
    stack.value.unshift(...removed)
    sortStack()

    if (adminView && removedBy !== null) {
      globalDialog.toast(`${escapeHTML(removedBy)} cleared the queue!`, 'error', 60000)
    } else {
      globalDialog.toast('The queue has been cleared for this session.', 'error', 60000)
    }
  }

  function handleEntryPinned() {
    // Notification to student they were pinned - matching original
    sendNotification(
      'You were pinned!',
      'Another staff member will be joining shortly!'
    )
    globalDialog.alert({
      title: 'Pinned!',
      message: `You were pinned on the queue! More help is on the way. You'll get a notification when you've been popped again.`,
      type: 'info',
      hasIcon: true,
    })
  }

  function handleEntryHelping(data: { helping: string }) {
    // Notification to student about being helped - matching original
    if (data.helping !== '') {
      sendNotification(
        'You are being helped!',
        `Please be ready for a staff member to join you!`
      )
      globalDialog.alert({
        title: `You're up!`,
        message: `${escapeHTML(data.helping)} is now coming to help you. Please be ready for them to join!`,
        type: 'success',
        hasIcon: true,
      })
    } else {
      sendNotification(
        'You are no longer being helped.',
        `A staff member indicated that they're no longer helping you.`
      )
      globalDialog.alert({
        title: 'No longer helping.',
        message: `A staff member indicated that they're no longer helping you. If you're not expecting this, make sure you're available for them!`,
        type: 'warning',
        hasIcon: true,
      })
    }
  }

  function handleNotHelped() {
    // Notification to student they couldn't be reached - matching original
    globalDialog.alert({
      title: `We Couldn't Find You!`,
      message: `A staff member attempted to help you, but they let us know that they weren't able to make contact with you. Please make sure your location is descriptive or your meeting link is still valid!` +
        (config.value?.prioritize_new
          ? `<br><br><b>This didn't count as your first meeting of the day.</b>`
          : ''),
      hasIcon: true,
      type: 'danger',
    })
  }

  function handleUserStatusUpdate(data: { email: string; status: string }) {
    const email = data.email
    const isOnline = data.status === 'online'

    // Update online set
    if (isOnline) {
      online.value.add(email)
    } else {
      online.value.delete(email)
    }

    // Update online status on all matching entries - matching original
    entries.value
      .filter(e => e.email === email)
      .forEach(e => {
        e.online = isOnline
      })
    stack.value
      .filter(e => e.email === email)
      .forEach(e => {
        e.online = isOnline
      })
  }

  function handleAnnouncementCreate(data: Announcement) {
    const existingIndex = announcements.value.findIndex(a => a.id === data.id)
    if (existingIndex !== -1) {
      announcements.value[existingIndex] = data
    } else {
      announcements.value.push(data)
    }
  }

  function handleAnnouncementDelete(data: { id: string }) {
    const index = announcements.value.findIndex(a => a.id === data.id)
    if (index !== -1) {
      announcements.value.splice(index, 1)
    }
  }

  function sortEntries() {
    // Match original sorting logic from OrderedQueue.ts
    entries.value.sort((a, b) => {
      // Pinned entries first
      if (a.pinned !== b.pinned) {
        return a.pinned ? -1 : 1
      }

      // Being helped entries second
      const aHelping = a.helping !== ''
      const bHelping = b.helping !== ''
      if (aHelping !== bHelping) {
        return aHelping ? -1 : 1
      }

      // Then by priority (higher priority first)
      if (a.priority !== b.priority) {
        return b.priority - a.priority
      }

      // Finally by ID (earlier first)
      return a.id < b.id ? -1 : a.id > b.id ? 1 : 0
    })
  }

  function sortStack() {
    // Match original sorting logic - by removed_at descending, then by id
    stack.value.sort((a, b) => {
      const aTime = moment(a.removed_at)
      const bTime = moment(b.removed_at)
      if (!aTime.isSame(bTime)) {
        return bTime.diff(aTime)
      }
      return a.id > b.id ? -1 : a.id < b.id ? 1 : 0
    })
  }

  // Store reference for WS to send PONG
  let wsRef: WebSocket | null = null

  function setWebSocket(ws: WebSocket | null) {
    wsRef = ws
  }

  // Main message dispatcher - matching original OrderedQueue.handleWSMessage
  function handleWebSocketMessage(type: string, data: unknown, courseName?: string, adminView = false) {
    switch (type) {
      case 'PING':
        wsRef?.send(JSON.stringify({ e: 'PONG' }))
        break

      case 'ENTRY_CREATE':
        handleEntryCreate(data as Record<string, unknown>, adminView)
        break

      case 'ENTRY_REMOVE':
        handleEntryRemove(data as Record<string, unknown>)
        break

      case 'ENTRY_UPDATE':
        handleEntryUpdate(data as Record<string, unknown>)
        break

      case 'ENTRY_PINNED':
        handleEntryPinned()
        break

      case 'ENTRY_HELPING':
        handleEntryHelping(data as { helping: string })
        break

      case 'STACK_REMOVE':
        handleStackRemove(data as { id: string })
        break

      case 'QUEUE_OPEN':
        handleQueueOpen(data as boolean)
        break

      case 'QUEUE_CLEAR':
        handleQueueClear(data as string | null, adminView)
        break

      case 'NOT_HELPED':
        handleNotHelped()
        break

      case 'USER_STATUS_UPDATE':
        handleUserStatusUpdate(data as { email: string; status: string })
        break

      case 'ANNOUNCEMENT_CREATE':
        handleAnnouncementCreate(data as Announcement)
        break

      case 'ANNOUNCEMENT_DELETE':
        handleAnnouncementDelete(data as { id: string })
        break

      case 'QUEUE_CONNECTIONS_UPDATE':
        websocketConnections.value = data as number
        break

      case 'MESSAGE_CREATE': {
        const msg = data as { receiver: string; content: string }
        const title = `Message from ${courseName || 'Staff'}`
        sendNotification(title, msg.content)
        globalDialog.alert({
          title,
          message: escapeHTML(msg.content),
          type: 'warning',
          hasIcon: true,
        })
        break
      }

      case 'QUEUE_RANDOMIZE':
        globalDialog.alert({
          title: 'Queue Randomized',
          message: 'The order of the queue was just randomized. The priorities on the queue now correspond to that randomization.',
          type: 'warning',
          hasIcon: true,
        })
        break

      case 'REFRESH': {
        const delay = Math.random() * 30000
        globalDialog.alert({
          title: 'Refreshing Shortly',
          message: `The server told me that we need to refresh the page to get new information. Refreshing shortly...`,
          type: 'warning',
          hasIcon: true,
        })
        setTimeout(() => window.location.reload(), delay)
        break
      }
    }
  }

  // Normalize entry with defaults to match original QueueEntry class behavior
  function normalizeEntry(data: Record<string, unknown>): QueueEntry {
    return {
      id: data.id as string,
      id_timestamp: data.id_timestamp as string,
      name: data.name as string | undefined,
      email: data.email as string | undefined,
      description: data.description as string | undefined,
      location: data.location as string | undefined,
      priority: (data.priority as number) || 0,
      pinned: (data.pinned as boolean) || false,
      helping: (data.helping as string) || '',
      helped: (data.helped as boolean) || false,
      online: (data.online as boolean) || false,
    }
  }

  // Normalize removed entry with defaults
  function normalizeRemovedEntry(data: Record<string, unknown>): RemovedQueueEntry {
    return {
      ...normalizeEntry(data),
      removed_at: data.removed_at as string,
      removed_by: data.removed_by as string,
    }
  }

  // Fetch queue information from API - single endpoint like original
  async function fetchQueueInfo(queueId: string) {
    setCurrentQueue(queueId)

    try {
      const res = await fetch(`/api/queues/${queueId}`)
      if (!res.ok) {
        console.error('Failed to fetch queue info:', res.status)
        return
      }

      const data = await res.json()

      // Set entries (API returns as 'queue') - normalize with defaults
      if (data.queue) {
        entries.value = data.queue.map((e: Record<string, unknown>) => normalizeEntry(e))
        sortEntries()
      }

      // Set stack - normalize with defaults
      if (data.stack) {
        stack.value = data.stack.map((e: Record<string, unknown>) => normalizeRemovedEntry(e))
        sortStack()
      }

      // Set open state
      if (data.open !== undefined) {
        queueOpen.value = data.open
      }

      // Set announcements
      if (data.announcements) {
        announcements.value = data.announcements
      }

      // Set config
      if (data.config) {
        config.value = data.config
      }

      // Set online users and update entries
      if (data.online) {
        online.value = new Set(data.online)
        // Update online status on entries - matching original
        online.value.forEach((email: string) => {
          entries.value
            .filter(e => e.email === email)
            .forEach(e => { e.online = true })
          stack.value
            .filter(e => e.email === email)
            .forEach(e => { e.online = true })
        })
      }

      // Set schedule if present
      if (data.schedule) {
        schedule.value = data.schedule
      }

      return data
    } catch (error) {
      console.error('Failed to fetch queue info:', error)
    }
  }

  return {
    // State
    currentQueueId,
    entries,
    stack,
    announcements,
    online,
    websocketConnections,
    queueOpen,
    config,
    schedule,
    isAdmin,
    // Getters
    sortedEntries,
    entryCount,
    isUserOnline,
    // Actions
    setCurrentQueue,
    setAdmin,
    fetchQueueInfo,
    handleWebSocketMessage,
    setWebSocket,
    sortEntries,
    sortStack,
  }
})
