import { ref, onMounted, onUnmounted } from 'vue'
import type { WSMessage } from '@/types'

export interface UseWebSocketOptions {
  onMessage?: (type: string, data: unknown) => void
  onConnect?: () => void
  onDisconnect?: (event: CloseEvent) => void
  reconnect?: boolean
  reconnectDelay?: number
}

export function useWebSocket(queueId: string, options: UseWebSocketOptions = {}) {
  const {
    onMessage,
    onConnect,
    onDisconnect,
    reconnect = true,
    reconnectDelay = 3000,
  } = options

  const ws = ref<WebSocket | null>(null)
  const connected = ref(false)
  const reconnectAttempts = ref(0)

  let reconnectTimeout: ReturnType<typeof setTimeout> | null = null

  function getWebSocketUrl(): string {
    const url = new URL(`/api/queues/${queueId}/ws`, window.location.href)
    url.protocol = url.protocol.replace('http', 'ws')
    return url.href
  }

  function connect() {
    if (ws.value?.readyState === WebSocket.OPEN) return

    try {
      ws.value = new WebSocket(getWebSocketUrl())

      ws.value.onopen = () => {
        connected.value = true
        reconnectAttempts.value = 0
        onConnect?.()
      }

      ws.value.onclose = (event) => {
        connected.value = false
        onDisconnect?.(event)

        if (reconnect && !event.wasClean) {
          scheduleReconnect()
        }
      }

      ws.value.onerror = () => {
        // Error will trigger onclose
      }

      ws.value.onmessage = (event) => {
        try {
          const msg: WSMessage = JSON.parse(event.data)

          // Handle ping/pong internally
          if (msg.e === 'PING') {
            send('PONG')
            return
          }

          onMessage?.(msg.e, msg.d)
        } catch {
          console.error('Failed to parse WebSocket message:', event.data)
        }
      }
    } catch (e) {
      console.error('Failed to create WebSocket:', e)
      if (reconnect) {
        scheduleReconnect()
      }
    }
  }

  function disconnect() {
    if (reconnectTimeout) {
      clearTimeout(reconnectTimeout)
      reconnectTimeout = null
    }

    if (ws.value) {
      ws.value.close()
      ws.value = null
    }

    connected.value = false
  }

  function scheduleReconnect() {
    if (reconnectTimeout) return

    reconnectAttempts.value++
    const delay = Math.min(reconnectDelay * reconnectAttempts.value, 30000)

    reconnectTimeout = setTimeout(() => {
      reconnectTimeout = null
      connect()
    }, delay)
  }

  function send(event: string, data?: unknown) {
    if (ws.value?.readyState === WebSocket.OPEN) {
      ws.value.send(JSON.stringify({ e: event, d: data }))
    }
  }

  onMounted(() => {
    connect()
  })

  onUnmounted(() => {
    disconnect()
  })

  return {
    ws,
    connected,
    reconnectAttempts,
    connect,
    disconnect,
    send,
  }
}
