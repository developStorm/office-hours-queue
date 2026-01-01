export interface ApiResponse<T> {
  data?: T
  error?: string
  status: number
}

export interface UserInfo {
  email: string
  first_name: string
  last_name: string
  site_admin: boolean
  admin_courses: string[]
}

export interface WSMessage {
  e: string  // event type
  d: unknown // data payload
}
