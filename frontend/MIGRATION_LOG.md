# Migration Log: Vue 2 + Buefy → Vue 3 + Vite + Tailwind CSS v4 + DaisyUI v5

## Overview

This document logs the migration attempt from the original Vue 2 + Vue CLI + Buefy frontend to Vue 3 + Vite + Tailwind CSS v4 + DaisyUI v5.

**Original Project**: `/Users/aether/dev/School/office-hours-queue/frontend`
**New Project**: `/Users/aether/dev/School/office-hours-queue-v2/frontend`

**Status**: INCOMPLETE - Migration abandoned due to numerous issues

---

## What Was Migrated

### Core Infrastructure
- Vue 3 with Composition API (`<script setup>`)
- Vite as build tool with WebSocket proxy to `https://queue.cs.stanford.edu`
- Pinia for state management (replacing Vuex-style global state)
- Tailwind CSS v4 with DaisyUI v5 (replacing Bulma/Buefy)
- TypeScript throughout

### Components Migrated
- `QueueEntry.vue` - Queue entry display with actions
- `QueueSignup.vue` - Queue signup form
- `OrderedQueue.vue` - Main queue display
- `QueueView.vue` - Queue page wrapper
- `AnnouncementDisplay.vue` - Announcement cards
- `Dialog.vue` - Modal dialogs and prompts
- `Toast.vue` - Toast notifications
- `AdminView.vue` - Admin panel
- `QueueManage.vue` - Queue settings modal

### Stores
- `queue.ts` - Queue state management with WebSocket handlers
- `user.ts` - User authentication state
- `app.ts` - Application state (courses, queues)

### Utilities
- `useDialog.ts` - Dialog/toast composable
- `notification.ts` - Browser notifications
- `ErrorDialog.ts` - Error handling
- `promptHandler.ts` - Custom prompts parsing
- `sanitization.ts` - HTML escaping

---

## Known Issues / Incomplete Items

### Business Logic Issues
1. **Entry normalization** - Had to add `normalizeEntry()` and `normalizeRemovedEntry()` functions to provide defaults like the original class constructors did
2. **WebSocket disconnections** - User reported frequent disconnections; root cause not fully investigated
3. **Helped/Not helped logic** - Multiple iterations to get the indicator and button conditions correct

### UI/Styling Issues
1. **Announcement color** - Fixed to `rgb(255, 224, 138)` but may still not match exactly
2. **Tooltip colors** - Added CSS overrides but may not match Buefy tooltips exactly
3. **Icon sizes** - Status icons changed to `w-10 h-10` (2.5rem) to match `is-size-2`
4. **Link underlines** - Added CSS but may not match Bulma exactly
5. **Overall scaling** - User reported UI "not scaled correctly"
6. **Various style mismatches** - Buttons, inputs, cards may not match Bulma/Buefy styling exactly

### Missing Features
1. **Schedule editor** - TODO placeholder
2. **CSV download** - TODO placeholder
3. **Appointments queue type** - Not implemented
4. **Document title updates** - Original updates title with queue position

### Untested/Unverified
1. All admin actions (pin, help, done, undo, not helped, message)
2. WebSocket reconnection behavior
3. Browser notifications
4. Mobile responsiveness
5. All edge cases in queue logic

---

## Files Changed

### Components
- `src/components/ordered/QueueEntry.vue`
- `src/components/ordered/QueueSignup.vue`
- `src/components/ordered/OrderedQueue.vue`
- `src/components/ui/Dialog.vue`
- `src/components/ui/Toast.vue`
- `src/components/AnnouncementDisplay.vue`
- `src/components/admin/QueueManage.vue`

### Views
- `src/views/QueueView.vue`
- `src/views/AdminView.vue`
- `src/views/HomeView.vue`

### Stores
- `src/stores/queue.ts` - Major rewrite with all WebSocket handlers
- `src/stores/user.ts`
- `src/stores/app.ts`

### Styles
- `src/assets/main.css` - Custom theme colors and overrides

### Types
- `src/types/QueueEntry.ts`
- `src/types/Queue.ts`
- `src/types/index.ts`

### Utilities
- `src/composables/useDialog.ts`
- `src/utils/notification.ts`
- `src/utils/ErrorDialog.ts`
- `src/utils/promptHandler.ts`
- `src/utils/sanitization.ts`

---

## Key Differences from Original

### Original (Vue 2 + Buefy)
- Class-based components with `vue-property-decorator`
- Global `$root.$data` for user info
- Buefy components (`b-tooltip`, `b-field`, `b-input`, etc.)
- FontAwesome icons via `@fortawesome/vue-fontawesome`
- Class-based models (`QueueEntry`, `RemovedQueueEntry`, `OrderedQueue`)
- Bulma CSS framework

### New (Vue 3 + DaisyUI)
- Composition API with `<script setup>`
- Pinia stores for state
- DaisyUI components (`.btn`, `.card`, `.input`, etc.)
- Lucide icons (`lucide-vue-next`)
- Plain TypeScript interfaces
- Tailwind CSS v4

---

## Recommendations for Future Work

1. **Start fresh** - The component-by-component migration approach led to many subtle bugs
2. **Test incrementally** - Each component should be fully tested before moving on
3. **Match original exactly** - Don't try to "improve" during migration
4. **Keep original classes** - The original class-based models (`QueueEntry`, etc.) provided defaults and computed properties that were lost in the plain interface approach
5. **Visual regression testing** - Screenshots of original vs new to catch styling issues early

---

## Commands

```bash
# Install dependencies
npm install

# Development server
npm run dev

# Build for production
npm run build

# Type check
npm run type-check
```

---

## Date
December 31, 2025
