export interface AppointmentSlot {
  scheduled_time: string
  timeslot: number
  duration: number
}

export interface Appointment extends AppointmentSlot {
  id: string
  id_timestamp: string
  name?: string
  student_email?: string
  staff_email?: string
  description?: string
  location?: string
}

// Helper functions
export function isSlotFilled(slot: AppointmentSlot | Appointment): boolean {
  return 'id' in slot
}

export function isFilledByStudent(appointment: Appointment): boolean {
  return appointment.student_email !== undefined || appointment.staff_email === undefined
}

export function isFilledByStaff(appointment: Appointment): boolean {
  return appointment.staff_email !== undefined
}

export function isSlotPast(slot: AppointmentSlot, now: Date): boolean {
  return new Date(slot.scheduled_time) < now
}
