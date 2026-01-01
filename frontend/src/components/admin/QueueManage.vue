<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { X } from 'lucide-vue-next'
import { globalDialog } from '@/composables/useDialog'
import { escapeHTML } from '@/utils/sanitization'

const props = defineProps<{
  type: 'ordered' | 'appointments'
  defaultConfiguration: Record<string, unknown>
  defaultGroups: string[][]
}>()

const emit = defineEmits<{
  close: []
  announcementAdded: [content: string]
  configurationSaved: [config: Record<string, unknown>, promptsInput: string]
  groupsSaved: [groups: string[][]]
}>()

const configuration = ref<Record<string, unknown>>({})
const groups = ref<string[][]>([])
const groupsInput = ref('')
const promptsInput = ref('')

const groupsPlaceholder = [
  ['member 1 of group 1', 'member 2 of group 1'],
  ['member 1 of group 2', 'member 2 of group 2', 'member 3 of group 2'],
  ['only member of group 3'],
]

onMounted(() => {
  configuration.value = { ...props.defaultConfiguration }
  promptsInput.value =
    Array.isArray(configuration.value.prompts) && (configuration.value.prompts as string[]).length > 0
      ? JSON.stringify(configuration.value.prompts, null, 2)
      : ''
  for (const group of props.defaultGroups) {
    groups.value.push([...group])
  }
})

function addAnnouncement() {
  globalDialog.prompt({
    title: 'Add Announcement',
    message: 'Announcement content:',
    confirmText: 'Add Announcement',
    onConfirm: (content) => emit('announcementAdded', content),
  })
}

function validateGroupsArray(obj: unknown, currentGroups: string[][]): boolean {
  if (!Array.isArray(obj)) {
    globalDialog.alert({
      title: 'Error',
      message: 'Input does not contain array of arrays of strings.',
      type: 'danger',
    })
    return false
  }
  const emailsSeen = new Set<string>()
  for (const g of currentGroups) {
    for (const e of g) {
      emailsSeen.add(e)
    }
  }
  for (const g of obj) {
    if (!Array.isArray(g)) {
      globalDialog.alert({
        title: 'Error',
        message: 'Input does not contain array of arrays of strings.',
        type: 'danger',
      })
      return false
    }
    for (const e of g) {
      if (typeof e !== 'string') {
        globalDialog.alert({
          title: 'Error',
          message: 'Input contains a non-string email.',
          type: 'danger',
        })
        return false
      }
      if (emailsSeen.has(e)) {
        globalDialog.alert({
          title: 'Error',
          message: `Email ${escapeHTML(e)} appears in more than one group!`,
          type: 'danger',
        })
        return false
      }
      emailsSeen.add(e)
    }
  }
  return true
}

function addGroups() {
  try {
    const parsed = JSON.parse(groupsInput.value)
    if (!validateGroupsArray(parsed, groups.value)) {
      return
    }
    for (const g of parsed) {
      groups.value.push([...g])
    }
  } catch {
    globalDialog.alert({
      title: 'Error',
      message: 'Input is not valid JSON.',
      type: 'danger',
    })
  }
}

function setGroups() {
  try {
    const parsed = JSON.parse(groupsInput.value)
    if (!validateGroupsArray(parsed, [])) {
      return
    }
    groups.value = []
    for (const g of parsed) {
      groups.value.push([...g])
    }
  } catch {
    globalDialog.alert({
      title: 'Error',
      message: 'Input is not valid JSON.',
      type: 'danger',
    })
  }
}

function removeGroup(i: number) {
  groups.value.splice(i, 1)
}

function removeMember(i: number, email: string) {
  const group = groups.value[i]
  if (!group) return
  groups.value[i] = group.filter((e) => e !== email)
  if (groups.value[i]?.length === 0) {
    removeGroup(i)
  }
}
</script>

<template>
  <div class="modal modal-open">
    <div class="modal-box max-w-3xl max-h-[90vh]">
      <div class="flex justify-between items-center mb-4">
        <h3 class="font-bold text-lg">Manage Queue</h3>
        <button class="btn btn-ghost btn-sm btn-circle" @click="emit('close')">
          <X class="w-4 h-4" />
        </button>
      </div>

      <div class="space-y-6 overflow-y-auto">
        <!-- Add Announcement -->
        <div>
          <button class="btn btn-primary" @click="addAnnouncement">
            Add Announcement
          </button>
        </div>

        <!-- Queue Settings -->
        <div>
          <h4 class="text-lg font-bold mb-4">Queue Settings</h4>
          <div class="space-y-2">
            <label class="label cursor-pointer justify-start gap-3">
              <input
                type="checkbox"
                v-model="configuration['virtual']"
                class="checkbox"
              />
              <span class="label-text">This queue is virtual (only changes UI)</span>
            </label>

            <label v-if="type === 'ordered'" class="label cursor-pointer justify-start gap-3">
              <input
                type="checkbox"
                v-model="configuration['scheduled']"
                class="checkbox"
              />
              <span class="label-text">Open and close queue according to schedule</span>
            </label>

            <label class="label cursor-pointer justify-start gap-3">
              <input
                type="checkbox"
                v-model="configuration['enable_location_field']"
                class="checkbox"
              />
              <span class="label-text">Allow students to specify location or meeting link</span>
            </label>

            <label class="label cursor-pointer justify-start gap-3">
              <input
                type="checkbox"
                v-model="configuration['prevent_unregistered']"
                class="checkbox"
              />
              <span class="label-text">Prevent students not registered in any group from signing up</span>
            </label>

            <label class="label cursor-pointer justify-start gap-3">
              <input
                type="checkbox"
                v-model="configuration['prevent_groups']"
                class="checkbox"
              />
              <span class="label-text">Prevent multiple students in a group from signing up at the same time</span>
            </label>

            <label v-if="type === 'ordered'" class="label cursor-pointer justify-start gap-3">
              <input
                type="checkbox"
                v-model="configuration['prioritize_new']"
                class="checkbox"
              />
              <span class="label-text">Prioritize students who signed up for the first time this day</span>
            </label>

            <label v-if="type === 'ordered'" class="label cursor-pointer justify-start gap-3">
              <input
                type="checkbox"
                v-model="configuration['prevent_groups_boost']"
                class="checkbox"
              />
              <span class="label-text">Prevent multiple students in a group from receiving the boost for first question per day</span>
            </label>
          </div>

          <div v-if="type === 'ordered'" class="form-control mt-4">
            <label class="label">
              <span class="label-text">Student signup cooldown after being helped (seconds)</span>
            </label>
            <input
              type="number"
              v-model.number="configuration['cooldown']"
              class="input input-bordered w-full max-w-xs"
            />
          </div>

          <div class="form-control mt-4">
            <label class="label">
              <span class="label-text">Custom Prompts (JSON array of prompt strings, leave empty for default)</span>
            </label>
            <textarea
              v-model="promptsInput"
              class="textarea textarea-bordered w-full h-24"
              :placeholder="JSON.stringify(['What milestone are you working on?', 'What have you tried?'], null, 2)"
            ></textarea>
          </div>

          <button
            class="btn btn-primary mt-4"
            @click="emit('configurationSaved', configuration, promptsInput)"
          >
            Save Queue Settings
          </button>
        </div>

        <!-- Queue Groups -->
        <div>
          <h4 class="text-lg font-bold mb-4">Queue Groups</h4>
          <div class="form-control">
            <label class="label">
              <span class="label-text">Groups input (JSON; array of groups, each group is array of emails)</span>
            </label>
            <textarea
              v-model="groupsInput"
              class="textarea textarea-bordered w-full h-40"
              :placeholder="JSON.stringify(groupsPlaceholder, null, 4)"
            ></textarea>
          </div>

          <div class="flex flex-wrap gap-2 mt-4">
            <button class="btn btn-primary" @click="addGroups">Add Groups</button>
            <button class="btn btn-warning" @click="setGroups">Overwrite Groups</button>
            <button class="btn btn-success" @click="emit('groupsSaved', groups)">Upload Groups</button>
          </div>

          <!-- Groups List -->
          <div class="mt-4">
            <h5 class="font-bold mb-2">Groups</h5>
            <div class="space-y-2">
              <div
                v-for="(group, i) in groups"
                :key="i"
                class="flex items-center gap-2 p-2 bg-base-200 rounded"
              >
                <button
                  class="btn btn-ghost btn-xs btn-circle"
                  @click="removeGroup(i)"
                >
                  <X class="w-3 h-3" />
                </button>
                <div class="flex flex-wrap gap-1">
                  <span
                    v-for="(email, j) in group"
                    :key="email"
                    class="badge badge-outline cursor-pointer hover:badge-error"
                    @click="removeMember(i, email)"
                  >
                    {{ email }}{{ j !== group.length - 1 ? ',' : '' }}
                  </span>
                </div>
              </div>
              <p v-if="groups.length === 0" class="text-base-content/60 italic">
                No groups defined
              </p>
            </div>
          </div>
        </div>
      </div>

      <div class="modal-action">
        <button class="btn" @click="emit('close')">Close</button>
      </div>
    </div>
    <div class="modal-backdrop" @click="emit('close')"></div>
  </div>
</template>
