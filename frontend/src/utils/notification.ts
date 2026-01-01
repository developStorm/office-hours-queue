function checkNotificationPromise(): boolean {
  try {
    Notification.requestPermission().then()
  } catch {
    return false
  }
  return true
}

export function sendNotification(title: string, body: string): void {
  if (!('Notification' in window)) return

  if (checkNotificationPromise()) {
    Notification.requestPermission().then((p) => {
      if (p === 'granted') {
        new Notification(title, { body })
      }
    })
  } else {
    // Legacy callback syntax for older browsers
    Notification.requestPermission((p) => {
      if (p === 'granted') {
        new Notification(title, { body })
      }
    })
  }
}
