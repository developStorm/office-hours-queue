<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { SquareArrowUp, SquareArrowLeft } from 'lucide-vue-next'
import { useAppStore } from '@/stores/app'

const appStore = useAppStore()
const windowWidth = ref(window.innerWidth)

const isMobile = computed(() => windowWidth.value <= 768)

function handleResize() {
  windowWidth.value = window.innerWidth
}

onMounted(() => {
  appStore.showCourses = true
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
})
</script>

<template>
  <div class="bg-primary text-primary-content rounded py-12 px-8">
    <!-- Mobile: pointing up -->
    <SquareArrowUp v-if="isMobile" class="w-24 h-24 mb-4" />
    <!-- Desktop: pointing left -->
    <SquareArrowLeft v-else class="w-24 h-24 mb-4" />
    <h1 class="text-3xl font-bold">Welcome to CS Office Hours!</h1>
    <p v-if="isMobile" class="py-2 opacity-90 text-lg">Select a course above to begin.</p>
    <p v-else class="py-2 opacity-90 text-lg">Select a course on the left to begin.</p>
  </div>
</template>
