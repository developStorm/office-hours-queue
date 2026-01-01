import type { Announcement } from './Announcement'

export interface QueueConfiguration {
  virtual?: boolean
  confirm_signup_message?: string
  enable_location_field?: boolean
  prevent_groups?: boolean
  prevent_groups_boost?: boolean
  prevent_unregistered?: boolean
  prioritize_new?: boolean
  scheduled?: boolean
  prompts?: string[]
}

// Minimal course reference to avoid circular imports
export interface QueueCourseRef {
  id: string
  short_name: string
  full_name: string
}

export interface Queue {
  id: string
  type: 'ordered' | 'appointments'
  name: string
  location: string
  map: string
  course_id?: string
  course?: QueueCourseRef
  config?: QueueConfiguration
  announcements?: Announcement[]
  open?: boolean
}

export interface QueueInfo extends Queue {
  announcements: Announcement[]
  config: QueueConfiguration
  online?: string[]
  schedule?: ScheduleDay[]
}

export interface ScheduleDay {
  day: string
  start: string
  end: string
  duration: number
}
