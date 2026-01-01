<script setup lang="ts">
import { ref, computed } from 'vue'
import { RouterLink } from 'vue-router'
import {
  Plus,
  Pencil,
  Trash2,
  Hand,
  Calendar,
  LogIn,
} from 'lucide-vue-next'
import type { Course, Queue } from '@/types'
import { useAppStore } from '@/stores/app'
import { useUserStore } from '@/stores/user'
import { globalDialog } from '@/composables/useDialog'
import ErrorDialog from '@/utils/ErrorDialog'
import { escapeHTML } from '@/utils/sanitization'
import CourseEdit from '@/components/admin/CourseEdit.vue'
import QueueAdd from '@/components/admin/QueueAdd.vue'

const appStore = useAppStore()
const userStore = useUserStore()

const showCourseEditModal = ref(false)
const showQueueAddModal = ref(false)
const editingCourseIndex = ref(-1)
const addingQueueCourseIndex = ref(-1)
const courseEditProps = ref({
  defaultShortName: '',
  defaultFullName: '',
  defaultAdmins: [] as string[],
})

const courses = computed(() => {
  if (!userStore.userInfo) return []
  return appStore.courseList.filter((c: Course) =>
    userStore.userInfo?.site_admin || userStore.userInfo?.admin_courses?.includes(c.id)
  )
})

const hasAdminAccess = computed(() => {
  if (!userStore.userInfo) return false
  return (
    userStore.userInfo.site_admin ||
    (userStore.userInfo.admin_courses && userStore.userInfo.admin_courses.length > 0)
  )
})

function loginWithRedirect() {
  localStorage.setItem('loginRedirect', '/admin')
  window.location.href = '/api/oauth2login'
}

function addCourse() {
  editingCourseIndex.value = -1
  courseEditProps.value = {
    defaultShortName: '',
    defaultFullName: '',
    defaultAdmins: [userStore.userInfo?.email || ''],
  }
  showCourseEditModal.value = true
}

async function editCourse(index: number) {
  const course = courses.value[index]
  if (!course) return
  try {
    const res = await fetch(`/api/courses/${course.id}/admins`)
    const admins = await res.json()
    editingCourseIndex.value = index
    courseEditProps.value = {
      defaultShortName: course.short_name,
      defaultFullName: course.full_name,
      defaultAdmins: admins,
    }
    showCourseEditModal.value = true
  } catch (e) {
    console.error('Failed to fetch course admins:', e)
  }
}

async function handleCourseSaved(shortName: string, fullName: string, admins: string[]) {
  showCourseEditModal.value = false

  if (editingCourseIndex.value === -1) {
    // Adding new course
    const res = await fetch('/api/courses', {
      method: 'POST',
      body: JSON.stringify({ short_name: shortName, full_name: fullName }),
    })
    if (res.status !== 201) {
      return ErrorDialog(res)
    }

    if (admins.length > 0) {
      const body = await res.json()
      const adminsRes = await fetch(`/api/courses/${body.id}/admins`, {
        method: 'PUT',
        body: JSON.stringify(admins),
      })
      if (adminsRes.status !== 204) {
        return ErrorDialog(adminsRes)
      }
    }
    location.reload()
  } else {
    // Editing existing course
    const course = courses.value[editingCourseIndex.value]
    if (!course) return
    const [courseUpdate, adminsUpdate] = await Promise.all([
      fetch(`/api/courses/${course.id}`, {
        method: 'PUT',
        body: JSON.stringify({ short_name: shortName, full_name: fullName }),
      }),
      fetch(`/api/courses/${course.id}/admins`, {
        method: 'PUT',
        body: JSON.stringify(admins),
      }),
    ])
    if (courseUpdate.status !== 204) {
      return ErrorDialog(courseUpdate)
    }
    if (adminsUpdate.status !== 204) {
      return ErrorDialog(adminsUpdate)
    }
    location.reload()
  }
}

async function deleteCourse(index: number) {
  const course = courses.value[index]
  if (!course) return
  const confirmed = await globalDialog.confirm({
    title: 'Delete Course',
    message: `Are you sure you want to delete ${escapeHTML(course.short_name)}? This will also delete all associated queues. <b>There is no undo.</b>`,
    type: 'danger',
    hasIcon: true,
    confirmText: 'Delete',
    cancelText: 'Cancel',
  })

  if (confirmed) {
    const res = await fetch(`/api/courses/${course.id}`, {
      method: 'DELETE',
    })
    if (res.status !== 204) {
      return ErrorDialog(res)
    }
    location.reload()
  }
}

function addQueue(index: number) {
  addingQueueCourseIndex.value = index
  showQueueAddModal.value = true
}

async function handleQueueSaved(name: string, loc: string, type: string) {
  showQueueAddModal.value = false
  const course = courses.value[addingQueueCourseIndex.value]
  if (!course) return

  const res = await fetch(`/api/courses/${course.id}/queues`, {
    method: 'POST',
    body: JSON.stringify({
      name,
      location: loc,
      type,
    }),
  })
  if (res.status !== 201) {
    return ErrorDialog(res)
  }
  location.reload()
}

async function deleteQueue(queue: Queue) {
  const confirmed = await globalDialog.confirm({
    title: 'Delete Queue',
    message: `Are you sure you want to delete ${escapeHTML(queue.name)}? <b>There is no undo.</b>`,
    type: 'danger',
    hasIcon: true,
    confirmText: 'Delete',
    cancelText: 'Cancel',
  })

  if (confirmed) {
    const res = await fetch(`/api/queues/${queue.id}`, {
      method: 'DELETE',
    })
    if (res.status !== 204) {
      return ErrorDialog(res)
    }
    location.reload()
  }
}
</script>

<template>
  <div v-if="userStore.loggedIn">
    <div v-if="hasAdminAccess" class="space-y-6">
      <h1 class="text-2xl font-bold">Courses</h1>

      <button
        v-if="userStore.userInfo?.site_admin"
        class="btn btn-primary w-full"
        @click="addCourse"
      >
        <Plus class="w-4 h-4" />
        Add Course
      </button>

      <!-- Course panels -->
      <div v-for="(course, i) in courses" :key="course.id" class="card bg-base-100 shadow">
        <div class="card-body p-0">
          <!-- Course header -->
          <div class="flex justify-between items-center p-4 bg-base-200 rounded-t-lg">
            <div class="flex items-center gap-4">
              <span class="font-bold">{{ course.short_name }}</span>
              <span class="text-base-content/70">{{ course.full_name }}</span>
            </div>
            <div class="flex items-center gap-1">
              <div class="tooltip" data-tip="Add Queue">
                <button class="btn btn-ghost btn-sm btn-circle" @click="addQueue(i)">
                  <Plus class="w-4 h-4" />
                </button>
              </div>
              <div class="tooltip" data-tip="Edit Course">
                <button class="btn btn-ghost btn-sm btn-circle" @click="editCourse(i)">
                  <Pencil class="w-4 h-4" />
                </button>
              </div>
              <div class="tooltip" data-tip="Delete Course">
                <button class="btn btn-ghost btn-sm btn-circle" @click="deleteCourse(i)">
                  <Trash2 class="w-4 h-4" />
                </button>
              </div>
            </div>
          </div>

          <!-- Queue list -->
          <div class="divide-y divide-base-200">
            <div
              v-for="queue in course.queues"
              :key="queue.id"
              class="flex items-center gap-3 p-3 hover:bg-base-100"
            >
              <button
                class="btn btn-ghost btn-sm btn-circle"
                @click="deleteQueue(queue)"
              >
                <Trash2 class="w-4 h-4" />
              </button>
              <Hand v-if="queue.type === 'ordered'" class="w-4 h-4 text-base-content/60" />
              <Calendar v-else-if="queue.type === 'appointments'" class="w-4 h-4 text-base-content/60" />
              <RouterLink :to="`/queues/${queue.id}`" class="link link-hover">
                {{ queue.name }}
              </RouterLink>
            </div>
            <div v-if="!course.queues || course.queues.length === 0" class="p-3 text-base-content/60 italic">
              No queues
            </div>
          </div>
        </div>
      </div>

      <div v-if="courses.length === 0" class="text-center text-base-content/60 py-8">
        No courses to manage
      </div>
    </div>

    <!-- No admin access -->
    <div v-else class="max-w-md mx-auto mt-8 p-4">
      <div class="card bg-base-100 shadow text-center">
        <div class="card-body">
          <h2 class="card-title justify-center">No Admin Access</h2>
          <p class="text-base-content/70">
            You don't have course or site admin privileges. Please contact us if
            you believe you should have access to this area (e.g. if you are a
            faculty member).
          </p>
          <div class="card-actions justify-center mt-4">
            <RouterLink to="/" class="btn btn-primary">
              Return to Home
            </RouterLink>
          </div>
        </div>
      </div>
    </div>
  </div>

  <!-- Not logged in -->
  <div v-else class="max-w-md mx-auto mt-8 p-4">
    <div class="card bg-base-100 shadow text-center">
      <div class="card-body">
        <h2 class="card-title justify-center">Login Required</h2>
        <p class="text-base-content/70">
          You need to be logged in with (site/course) admin privileges to manage
          courses and queues.
        </p>
        <div class="card-actions justify-center mt-4">
          <button @click="loginWithRedirect" class="btn btn-primary">
            <LogIn class="w-4 h-4" />
            Log In
          </button>
        </div>
      </div>
    </div>
  </div>

  <!-- Modals -->
  <CourseEdit
    v-if="showCourseEditModal"
    v-bind="courseEditProps"
    @close="showCourseEditModal = false"
    @saved="handleCourseSaved"
  />

  <QueueAdd
    v-if="showQueueAddModal"
    @close="showQueueAddModal = false"
    @saved="handleQueueSaved"
  />
</template>
