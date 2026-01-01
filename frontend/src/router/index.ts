import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '@/views/HomeView.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'home',
      component: HomeView,
    },
    {
      path: '/queues/:qid',
      name: 'queue',
      component: () => import('@/views/QueueView.vue'),
    },
    {
      path: '/admin',
      name: 'admin',
      component: () => import('@/views/AdminView.vue'),
    },
  ],
})

router.beforeEach((to, _from, next) => {
  if (to.name === 'home') {
    document.title = 'Office Hours Queue'
  }
  next()
})

export default router
