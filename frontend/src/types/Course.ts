import type { Queue } from './Queue'

export interface Course {
  id: string
  short_name: string
  full_name: string
  queues: Queue[]
  favorite?: boolean
}
