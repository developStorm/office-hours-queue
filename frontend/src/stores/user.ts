import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { UserInfo } from '@/types'

export const useUserStore = defineStore('user', () => {
  // State
  const userInfoLoaded = ref(false)
  const loggedIn = ref(false)
  const userInfo = ref<UserInfo | null>(null)

  // Getters
  const isAdmin = computed(() => {
    if (!loggedIn.value || !userInfo.value) return false
    return userInfo.value.site_admin || (userInfo.value.admin_courses?.length ?? 0) > 0
  })

  const isSiteAdmin = computed(() => {
    return loggedIn.value && userInfo.value?.site_admin === true
  })

  const adminCourses = computed(() => {
    return userInfo.value?.admin_courses ?? []
  })

  // Actions
  async function fetchUserInfo() {
    try {
      const res = await fetch('/api/users/@me')
      userInfoLoaded.value = true

      if (!res.ok) {
        loggedIn.value = false
        userInfo.value = null
        return
      }

      const data = await res.json()
      loggedIn.value = true
      userInfo.value = data
    } catch (e) {
      console.error('Failed to fetch user info:', e)
      userInfoLoaded.value = true
      loggedIn.value = false
      userInfo.value = null
    }
  }

  function logout() {
    loggedIn.value = false
    userInfo.value = null
  }

  function isCourseAdmin(courseId: string): boolean {
    if (!loggedIn.value || !userInfo.value) return false
    if (userInfo.value.site_admin) return true
    return userInfo.value.admin_courses?.includes(courseId) ?? false
  }

  return {
    // State
    userInfoLoaded,
    loggedIn,
    userInfo,
    // Getters
    isAdmin,
    isSiteAdmin,
    adminCourses,
    // Actions
    fetchUserInfo,
    logout,
    isCourseAdmin,
  }
})
