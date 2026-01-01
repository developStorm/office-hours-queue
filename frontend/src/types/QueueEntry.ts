export interface QueueEntry {
  id: string
  id_timestamp: string
  created_at?: string
  name?: string
  email?: string
  description?: string
  location?: string
  priority: number
  pinned: boolean
  helping: string
  helped: boolean
  online: boolean
}

export interface RemovedQueueEntry extends QueueEntry {
  removed_at: string
  removed_by: string
}

// Helper functions for queue entries
export function isBeingHelped(entry: QueueEntry): boolean {
  return entry.helping !== ''
}

export function humanizedTimestamp(timestamp: string, now: Date): string {
  const then = new Date(timestamp)
  const diffMs = now.getTime() - then.getTime()
  const diffMins = Math.floor(diffMs / 60000)

  if (diffMins < 1) return 'just now'
  if (diffMins === 1) return '1 minute ago'
  if (diffMins < 60) return `${diffMins} minutes ago`

  const diffHours = Math.floor(diffMins / 60)
  if (diffHours === 1) return '1 hour ago'
  if (diffHours < 24) return `${diffHours} hours ago`

  const diffDays = Math.floor(diffHours / 24)
  if (diffDays === 1) return '1 day ago'
  return `${diffDays} days ago`
}

export function formatTimestamp(timestamp: string): string {
  return new Date(timestamp).toLocaleString()
}
