import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Course, Queue } from '@/types'

export const useAppStore = defineStore('app', () => {
  // State
  const courses = ref<Record<string, Course>>({})
  const queues = ref<Record<string, Queue>>({})
  const showCourses = ref(true)
  const studentView = ref(false)
  const favoriteCourseIds = ref<string[]>(loadFavoriteCourseIds())

  function loadFavoriteCourseIds(): string[] {
    const prefix = 'favoriteCourses-'
    const ids: string[] = []
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i)
      if (key && key.startsWith(prefix)) {
        ids.push(key.substring(prefix.length))
      }
    }
    return ids
  }

  // Getters
  const courseList = computed(() => Object.values(courses.value))
  const queueList = computed(() => Object.values(queues.value))

  // Actions
  async function fetchCourses() {
    try {
      const res = await fetch('/api/courses')
      if (!res.ok) return

      const data: Course[] = await res.json()
      courses.value = {}
      for (const course of data) {
        const isFavorite = favoriteCourseIds.value.includes(course.id)
        courses.value[course.id] = { ...course, favorite: isFavorite }
        // Also index queues with course reference
        for (const queue of course.queues || []) {
          queue.course = {
            id: course.id,
            short_name: course.short_name,
            full_name: course.full_name,
          }
          queues.value[queue.id] = queue
        }
      }
    } catch (e) {
      console.error('Failed to fetch courses:', e)
    }
  }

  function toggleFavorite(courseId: string) {
    const prefix = 'favoriteCourses-'
    const isFavorite = favoriteCourseIds.value.includes(courseId)

    if (isFavorite) {
      favoriteCourseIds.value = favoriteCourseIds.value.filter((id) => id !== courseId)
      localStorage.removeItem(prefix + courseId)
    } else {
      favoriteCourseIds.value = [...favoriteCourseIds.value, courseId]
      localStorage.setItem(prefix + courseId, 'favorite')
    }

    if (courses.value[courseId]) {
      courses.value[courseId] = { ...courses.value[courseId], favorite: !isFavorite }
    }
  }

  function setCourse(course: Course) {
    courses.value[course.id] = course
  }

  function setQueue(queue: Queue) {
    queues.value[queue.id] = queue
  }

  function getQueue(id: string): Queue | undefined {
    return queues.value[id]
  }

  return {
    // State
    courses,
    queues,
    showCourses,
    studentView,
    favoriteCourseIds,
    // Getters
    courseList,
    queueList,
    // Actions
    fetchCourses,
    toggleFavorite,
    setCourse,
    setQueue,
    getQueue,
  }
})
