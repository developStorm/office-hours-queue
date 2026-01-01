<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { RouterView, RouterLink, useRoute, useRouter } from 'vue-router'
import {
  ChevronLeft,
  ChevronRight,
  LogIn,
  LogOut,
  GraduationCap,
  ShieldCheck,
  Github,
  Star,
  BarChart3,
} from 'lucide-vue-next'
import { useAppStore } from '@/stores/app'
import { useUserStore } from '@/stores/user'
import ToastContainer from '@/components/ui/ToastContainer.vue'
import Dialog from '@/components/ui/Dialog.vue'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const userStore = useUserStore()

const fetchedCourses = ref(false)

// Computed
const courses = computed(() => {
  return appStore.courseList
    .filter(c => c.queues && c.queues.length > 0)
    .sort((a, b) => {
      if (a.favorite !== b.favorite) return a.favorite ? -1 : 1
      return a.short_name < b.short_name ? -1 : 1
    })
})

const admin = computed(() => {
  return !appStore.studentView && userStore.isAdmin
})

const siteAdmin = computed(() => {
  return !appStore.studentView && userStore.isSiteAdmin
})

// Methods
function goToQueue(queueId: string) {
  router.push(`/queues/${queueId}`)
}

function goHome() {
  appStore.showCourses = true
  router.push('/')
}

function toggleFavorite(course: { id: string; favorite?: boolean }) {
  appStore.toggleFavorite(course.id)
}

function isQueueActive(queueId: string) {
  return route.path.includes(queueId)
}

function isCourseActive(queues: { id: string }[]) {
  return queues.some(q => route.path.includes(q.id))
}

// Initialize
onMounted(async () => {
  if ('Notification' in window) {
    Notification.requestPermission()
  }

  await Promise.all([
    appStore.fetchCourses(),
    userStore.fetchUserInfo(),
  ])

  fetchedCourses.value = true
})
</script>

<template>
  <div class="min-h-screen bg-base-100">
    <!-- Container for nav, content, footer - enables sticky footer -->
    <div class="min-h-screen flex flex-col px-4 md:w-[80vw] max-w-8xl mx-auto">
      <!-- Navbar -->
      <nav class="border-b border-base-300 py-6">
        <div class="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <!-- Title -->
          <a href="/" class="text-3xl font-[500]" @click.prevent="goHome">
            CS Office Hours
          </a>

          <!-- Icons row -->
          <div class="flex items-center gap-1 justify-between md:justify-end">
            <div class="flex items-center gap-1">
              <!-- Student View Toggle -->
              <button
                v-if="admin"
                class="btn btn-ghost btn-sm"
                title="Student View"
                @click="appStore.studentView = true"
              >
                <GraduationCap class="w-6 h-6" />
              </button>
              <button
                v-if="appStore.studentView"
                class="btn btn-ghost btn-sm"
                title="Exit Student View"
                @click="appStore.studentView = false"
              >
                <ShieldCheck class="w-6 h-6" />
              </button>

              <!-- Admin Panel -->
              <RouterLink
                v-if="admin"
                to="/admin"
                class="btn btn-ghost btn-sm"
                title="Admin Panel"
              >
                <ShieldCheck class="w-6 h-6" />
              </RouterLink>

              <!-- Site Admin Logs -->
              <a
                v-if="siteAdmin"
                href="/kibana"
                target="_blank"
                class="btn btn-ghost btn-sm"
                title="System Logs"
              >
                <BarChart3 class="w-6 h-6" />
              </a>

              <!-- GitHub -->
              <a
                href="https://github.com/developStorm/office-hours-queue"
                target="_blank"
                class="btn btn-ghost btn-sm"
              >
                <Github class="w-6 h-6" />
              </a>
            </div>

            <!-- Login/Logout -->
            <span v-if="!userStore.userInfoLoaded" class="btn btn-info btn-sm loading">
              Log in
            </span>
            <a
              v-else-if="!userStore.loggedIn"
              href="/api/oauth2login"
              class="btn btn-info btn-sm gap-2"
            >
              <LogIn class="w-4 h-4" />
              Log in
            </a>
            <a
              v-else
              href="/api/logout"
              class="btn btn-error btn-sm gap-2"
            >
              <LogOut class="w-4 h-4" />
              Log out
            </a>
          </div>
        </div>
      </nav>

      <!-- Main Section -->
      <section class="flex-1 py-12">
        <div v-if="fetchedCourses" class="flex flex-col md:flex-row gap-4 md:gap-6">
          <!-- Sidebar: Collapsed state (desktop only) -->
          <aside v-if="!appStore.showCourses" class="hidden md:block w-6 flex-shrink-0">
            <button
              class="btn btn-ghost btn-xs"
              @click="appStore.showCourses = true"
            >
              <ChevronRight class="w-4 h-4" />
            </button>
          </aside>

          <!-- Sidebar: Expanded state -->
          <aside v-if="appStore.showCourses" class="w-full md:w-40 flex-shrink-0">
            <div class="md:sticky md:top-6">
              <!-- COURSES header with collapse button -->
              <div class="flex items-center justify-between mb-3">
                <span class="text-xs font-semibold tracking-wider text-base-content/60 uppercase">
                  Courses
                </span>
                <button
                  class="btn btn-ghost btn-xs"
                  @click="appStore.showCourses = false"
                >
                  <ChevronLeft class="w-4 h-4" />
                </button>
              </div>

              <!-- Course list -->
              <ul class="space-y-0">
                <li v-for="course in courses" :key="course.id">
                  <!-- Multi-queue course (expandable) -->
                  <details v-if="course.queues.length > 1" :open="isCourseActive(course.queues)">
                    <summary
                      class="flex items-center justify-between px-3 py-2 text-base rounded cursor-pointer hover:bg-base-200"
                      :class="{ 'bg-primary text-primary-content': isCourseActive(course.queues) }"
                    >
                      <span>{{ course.short_name }}</span>
                      <button
                        @click.stop="toggleFavorite(course)"
                        class="opacity-60 hover:opacity-100"
                      >
                        <Star
                          class="w-4 h-4"
                          :class="course.favorite ? 'fill-current' : ''"
                        />
                      </button>
                    </summary>
                    <ul class="ml-4 mt-1 space-y-0">
                      <li v-for="queue in course.queues" :key="queue.id">
                        <a
                          :href="`/queues/${queue.id}`"
                          class="block px-3 py-2 text-base rounded hover:bg-base-200"
                          :class="{ 'bg-primary text-primary-content': isQueueActive(queue.id) }"
                          @click.prevent="goToQueue(queue.id)"
                        >
                          {{ queue.name }}
                        </a>
                      </li>
                    </ul>
                  </details>

                  <!-- Single-queue course -->
                  <a
                    v-else
                    :href="`/queues/${course.queues[0]?.id}`"
                    class="flex items-center justify-between px-3 py-2 text-base rounded hover:bg-base-200"
                    :class="{ 'bg-primary text-primary-content': isCourseActive(course.queues) }"
                    @click.prevent="course.queues[0] && goToQueue(course.queues[0].id)"
                  >
                    <span>{{ course.short_name }}</span>
                    <button
                      @click.stop="toggleFavorite(course)"
                      class="opacity-60 hover:opacity-100"
                    >
                      <Star
                        class="w-4 h-4"
                        :class="course.favorite ? 'fill-current' : ''"
                      />
                    </button>
                  </a>
                </li>
              </ul>
            </div>
          </aside>

          <!-- Main Content -->
          <main class="flex-1 min-w-0 p-3">
            <RouterView :student-view="appStore.studentView" />
          </main>
        </div>

        <!-- Loading state -->
        <div v-else class="flex justify-center items-center h-64">
          <span class="loading loading-spinner loading-lg"></span>
        </div>
      </section>

      <!-- Footer -->
      <footer class="bg-base-200 py-6 text-center text-sm text-base-content/70">
        <p class="mb-2">
          Interested in using this for your class?
          <a
            href="https://forms.gle/1CPmifer8WjyfgXP6"
            target="_blank"
            class="link link-primary"
          >Fill out this form!</a>
        </p>
        <p>
          Created by
          <a
            href="https://github.com/CarsonHoffman/office-hours-queue"
            target="_blank"
            class="link"
          >Carson Hoffman</a>
          at University of Michigan. Operated by
          <a
            href="mailto:cs-oh-queue-dev@lists.stanford.edu"
            class="link"
          >cs-oh-queue-dev</a>
          and CSD-CF at Stanford.
        </p>
      </footer>
    </div>

    <!-- Global UI components -->
    <ToastContainer />
    <Dialog />
  </div>
</template>
