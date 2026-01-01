export function useNotification() {
  const isSupported = 'Notification' in window

  async function requestPermission(): Promise<NotificationPermission> {
    if (!isSupported) return 'denied'

    try {
      return await Notification.requestPermission()
    } catch {
      // Fallback for older browsers
      return new Promise((resolve) => {
        Notification.requestPermission((permission) => {
          resolve(permission)
        })
      })
    }
  }

  function send(title: string, body: string): Notification | null {
    if (!isSupported) return null

    if (Notification.permission === 'granted') {
      return new Notification(title, { body })
    }

    // Try to request permission and send
    requestPermission().then((permission) => {
      if (permission === 'granted') {
        new Notification(title, { body })
      }
    })

    return null
  }

  return {
    isSupported,
    requestPermission,
    send,
  }
}
