<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { Moment } from 'moment-timezone'
import {
  HelpCircle,
  Link,
  MapPin,
  UserPlus,
  Check,
  Pencil,
} from 'lucide-vue-next'
import type { Queue, QueueConfiguration } from '@/types'
import { useUserStore } from '@/stores/user'
import { useQueueStore } from '@/stores/queue'
import { globalDialog } from '@/composables/useDialog'
import ErrorDialog from '@/utils/ErrorDialog'
import { escapeHTML } from '@/utils/sanitization'
import * as PromptHandler from '@/utils/promptHandler'

const props = defineProps<{
  queue: Queue
  time: Moment
}>()

const userStore = useUserStore()
const queueStore = useQueueStore()

const description = ref('')
const location = ref('')
const customResponses = ref<string[]>([])

// Character limits matching server-side validation
const maxDescriptionLength = 1500
const maxLocationLength = 300

const config = computed(() => queueStore.config as QueueConfiguration | null)

const hasCustomPrompts = computed(() => PromptHandler.hasCustomPrompts(config.value ?? undefined))

const prompts = computed(() => config.value?.prompts || [])

// Initialize customResponses when prompts change
watch(prompts, (newPrompts) => {
  if (newPrompts.length > 0 && customResponses.value.length !== newPrompts.length) {
    customResponses.value = new Array(newPrompts.length).fill('')
  }
}, { immediate: true })

const totalCustomResponseLength = computed(() => {
  if (!customResponses.value.length) return 0
  return PromptHandler.responsesToDescription(customResponses.value).length
})

const isDescriptionTooLong = computed(() => {
  return hasCustomPrompts.value
    ? totalCustomResponseLength.value > maxDescriptionLength
    : description.value.length > maxDescriptionLength
})

const isLocationTooLong = computed(() => {
  return location.value.length > maxLocationLength
})

const myEntryIndex = computed(() => {
  const email = userStore.userInfo?.email
  if (!email) return -1
  return queueStore.entries.findIndex(e => e.email === email)
})

const myEntry = computed(() => {
  if (myEntryIndex.value === -1) return null
  return queueStore.entries[myEntryIndex.value]
})

const myEntryModified = computed(() => {
  const e = myEntry.value
  if (!e) return false

  if (hasCustomPrompts.value) {
    try {
      const currentDesc = PromptHandler.responsesToDescription(customResponses.value)
      return currentDesc !== e.description || e.location !== location.value
    } catch {
      return true
    }
  }

  return e.description !== description.value || e.location !== location.value
})

const isValidDescription = computed(() => {
  return hasCustomPrompts.value
    ? PromptHandler.areResponsesValid(customResponses.value, prompts.value)
    : description.value.trim() !== ''
})

const canSignUp = computed(() => {
  return (
    myEntry.value === null &&
    userStore.loggedIn &&
    queueStore.queueOpen &&
    isValidDescription.value &&
    (location.value.trim() !== '' || !config.value?.enable_location_field) &&
    !isDescriptionTooLong.value &&
    !isLocationTooLong.value
  )
})

// Watch for entry changes to update form
watch(myEntry, (newEntry) => {
  if (newEntry) {
    if (hasCustomPrompts.value) {
      customResponses.value = PromptHandler.descriptionToResponses(newEntry.description)
      // Ensure array length matches prompts
      while (customResponses.value.length < prompts.value.length) {
        customResponses.value.push('')
      }
    } else {
      description.value = newEntry.description || ''
    }
    location.value = newEntry.location || ''
  }
}, { immediate: true })

function getDescriptionValue(): string {
  return hasCustomPrompts.value
    ? PromptHandler.responsesToDescription(customResponses.value)
    : description.value
}

async function signUp() {
  if (config.value?.confirm_signup_message) {
    const confirmed = await globalDialog.confirm({
      title: 'Sign Up',
      message: escapeHTML(config.value.confirm_signup_message),
      type: 'warning',
      hasIcon: true,
      confirmText: 'Sign Up',
      cancelText: 'Cancel',
    })
    if (!confirmed) return
  }

  await signUpRequest()
}

async function signUpRequest() {
  const locationValue = config.value?.enable_location_field
    ? location.value
    : '(disabled)'

  const res = await fetch(`/api/queues/${props.queue.id}/entries`, {
    method: 'POST',
    body: JSON.stringify({
      description: getDescriptionValue(),
      location: locationValue,
    }),
  })

  if (res.status !== 201) {
    return ErrorDialog(res)
  }

  globalDialog.toast(
    `You're on the queue, ${escapeHTML(userStore.userInfo?.first_name || '')}!`,
    'success'
  )
}

async function updateRequest() {
  if (!myEntry.value) return

  const res = await fetch(
    `/api/queues/${props.queue.id}/entries/${myEntry.value.id}`,
    {
      method: 'PUT',
      body: JSON.stringify({
        description: getDescriptionValue(),
        location: location.value,
      }),
    }
  )

  if (res.status !== 204) {
    return ErrorDialog(res)
  }

  globalDialog.toast('Your request has been updated!', 'success')
}
</script>

<template>
  <div class="space-y-4">
    <!-- Custom prompts (when configured) -->
    <template v-if="hasCustomPrompts">
      <div v-for="(prompt, i) in prompts" :key="i" class="form-control">
        <label class="label">
          <span class="label-text font-semibold whitespace-normal break-words">{{ prompt }}</span>
        </label>
        <div class="relative">
          <span class="absolute left-3 top-1/2 -translate-y-1/2 z-10 text-base-content/50 pointer-events-none">
            <HelpCircle class="w-4 h-4" />
          </span>
          <input
            v-model="customResponses[i]"
            type="text"
            class="input input-bordered w-full pl-10"
            :class="{ 'input-error': isDescriptionTooLong }"
          />
        </div>
      </div>
      <label v-if="isDescriptionTooLong" class="label">
        <span class="label-text-alt text-error">
          Characters: {{ totalCustomResponseLength }}/{{ maxDescriptionLength }}
        </span>
      </label>
    </template>

    <!-- Default description field (when no custom prompts) -->
    <div v-else class="form-control">
      <label class="label">
        <span class="label-text">Description</span>
      </label>
      <div class="relative">
        <span class="absolute left-3 top-1/2 -translate-y-1/2 z-10 text-base-content/50 pointer-events-none">
          <HelpCircle class="w-4 h-4" />
        </span>
        <input
          v-model="description"
          type="text"
          class="input input-bordered w-full pl-10"
          :class="{ 'input-error': isDescriptionTooLong }"
          placeholder="Help us help you—please be descriptive!"
        />
      </div>
      <label v-if="isDescriptionTooLong" class="label">
        <span class="label-text-alt text-error">
          Characters: {{ description.length }}/{{ maxDescriptionLength }}
        </span>
      </label>
    </div>

    <!-- Location field -->
    <div v-if="config === null || config.enable_location_field" class="form-control">
      <label class="label">
        <span class="label-text" v-if="config === null">
          <span class="skeleton w-20 h-4"></span>
        </span>
        <span class="label-text" v-else-if="!config.virtual">Location</span>
        <span class="label-text" v-else>Meeting Link</span>
      </label>
      <div class="relative">
        <span class="absolute left-3 top-1/2 -translate-y-1/2 z-10 text-base-content/50 pointer-events-none">
          <span v-if="config === null" class="skeleton w-4 h-4"></span>
          <MapPin v-else-if="!config.virtual" class="w-4 h-4" />
          <Link v-else class="w-4 h-4" />
        </span>
        <input
          v-model="location"
          type="text"
          class="input input-bordered w-full pl-10"
          :class="{ 'input-error': isLocationTooLong }"
        />
      </div>
      <label v-if="isLocationTooLong" class="label">
        <span class="label-text-alt text-error">
          Characters: {{ location.length }}/{{ maxLocationLength }}
        </span>
      </label>
    </div>

    <!-- Sign up / Update buttons -->
    <div class="flex items-center gap-4">
      <!-- Sign up button -->
      <button
        v-if="myEntry === null"
        class="btn btn-success gap-2"
        :disabled="!canSignUp"
        @click="signUp"
      >
        <UserPlus class="w-4 h-4" />
        Sign Up
      </button>

      <!-- Update button -->
      <button
        v-else-if="myEntryModified"
        class="btn btn-warning gap-2"
        :disabled="isDescriptionTooLong || isLocationTooLong"
        @click="updateRequest"
      >
        <Pencil class="w-4 h-4" />
        Update Request
      </button>

      <!-- On queue indicator -->
      <button
        v-else
        class="btn btn-success gap-2"
        disabled
      >
        <Check class="w-4 h-4" />
        On queue at position #{{ myEntryIndex + 1 }}
      </button>

      <!-- Login prompt -->
      <p v-if="!userStore.loggedIn" class="text-sm text-base-content/60">
        Log in to sign up!
      </p>
    </div>
  </div>
</template>
