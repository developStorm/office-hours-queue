<template>
	<div>
		<template
			v-if="
				queue.config && queue.config.prompts && queue.config.prompts.length > 0
			"
		>
			<div class="field" v-for="(prompt, i) in queue.config.prompts" :key="i">
				<label class="label">{{ prompt }}</label>
				<div class="control has-icons-left">
					<input class="input" v-model="customResponses[i]" type="text" />
					<span class="icon is-small is-left">
						<font-awesome-icon icon="question" />
					</span>
				</div>
			</div>
		</template>
		<div class="field" v-else>
			<label class="label">Description</label>
			<div class="control has-icons-left">
				<input
					class="input"
					v-model="description"
					type="text"
					placeholder="Help us help you—please be descriptive!"
				/>
				<span class="icon is-small is-left">
					<font-awesome-icon icon="question" />
				</span>
			</div>
		</div>
		<div
			class="field"
			v-if="queue.config === null || queue.config.enableLocationField"
		>
			<label class="label" v-if="queue.config === null"
				><b-skeleton width="7em"
			/></label>
			<label class="label" v-else-if="!queue.config.virtual">Location</label>
			<label class="label" v-else>Meeting Link</label>
			<div class="control has-icons-left">
				<input class="input" v-model="location" type="text" />
				<span class="icon is-small is-left">
					<b-skeleton
						position="is-centered"
						width="1em"
						v-if="queue.config === null"
					/>
					<font-awesome-icon
						icon="map-marker"
						v-else-if="!queue.config.virtual"
					/>
					<font-awesome-icon icon="link" v-else />
				</span>
			</div>
		</div>
		<div class="field">
			<div class="control level-left">
				<button
					class="button is-success level-item"
					:disabled="!canSignUp"
					@click="signUp"
					v-if="myEntry === null"
				>
					<span class="icon"><font-awesome-icon icon="user-plus"/></span>
					<span>Sign Up</span>
				</button>
				<button
					class="button is-warning level-item"
					@click="updateRequest"
					v-else-if="myEntryModified"
				>
					<span class="icon"><font-awesome-icon icon="edit"/></span>
					<span>Update Request</span>
				</button>
				<button class="button is-success level-item" disabled="true" v-else>
					<span class="icon"><font-awesome-icon icon="check"/></span>
					<span>On queue at position #{{ this.myEntryIndex + 1 }}</span>
				</button>
				<p class="level-item" v-if="!$root.$data.loggedIn">
					Log in to sign up!
				</p>
			</div>
		</div>
	</div>
</template>

<script lang="ts">
import Vue from 'vue';
import { Moment } from 'moment';
import { Component, Prop, Watch } from 'vue-property-decorator';
import OrderedQueue from '@/types/OrderedQueue';
import { QueueEntry } from '@/types/QueueEntry';
import ErrorDialog from '@/util/ErrorDialog';
import EscapeHTML from '@/util/Sanitization';

import { library } from '@fortawesome/fontawesome-svg-core';
import {
	faUser,
	faQuestion,
	faLink,
	faUserPlus,
	faCheck,
	faEdit,
	faMapMarker,
} from '@fortawesome/free-solid-svg-icons';

library.add(
	faUser,
	faQuestion,
	faLink,
	faUserPlus,
	faCheck,
	faEdit,
	faMapMarker
);

@Component
export default class QueueSignup extends Vue {
	description = '';
	location = '';
	customResponses: string[] = [];

	@Prop({ required: true }) queue!: OrderedQueue;
	@Prop({ required: true }) time!: Moment;

	private hasDescWithPrompts(): boolean {
		return Boolean(this.queue.config?.prompts?.length);
	}

	private descWithPromptsToDescription(): string {
		return JSON.stringify(
			Object.fromEntries(
				this!.queue!.config!.prompts!.map((p, i) => [
					p,
					this.customResponses[i],
				])
			)
		);
	}

	private descriptionToDescWithPrompts(description: string): string[] {
		try {
			const parsedDescription = JSON.parse(description || '{}');
			return this!.queue!.config!.prompts!.map(
				(prompt) => parsedDescription[prompt] || ''
			);
		} catch (e) {
			return this!.queue!.config!.prompts!.map(() => '');
		}
	}

	@Watch('myEntry')
	myEntryUpdated(newEntry: QueueEntry | null) {
		if (newEntry !== null) {
			if (this.hasDescWithPrompts()) {
				this.customResponses = this.descriptionToDescWithPrompts(
					newEntry.description || ''
				);
			} else {
				this.description = newEntry.description || '';
			}
			this.location = newEntry.location || '';
		}
	}

	get canSignUp(): boolean {
		// Do not change the order of the expressions in this boolean
		// expression. Because myEntry is a computed property, it seems
		// that it has to be calculated at least once in order to be
		// reactive, which means that putting it at the end of the
		// expression means it isn't calculated until all of the previous
		// parts of the expression are true, which is only calculated when
		// deemed necessary based on reactivity. Thus, if one of the previous
		// parts of the expression return false on the first calculation,
		// we aren't reactive on myEntry until one of the previous
		// parts had a reactive update. This took way too long to figure out :(

		const isValidDescription = this.hasDescWithPrompts()
			? this.customResponses.every((r) => r.trim() !== '')
			: this.description.trim() !== '';

		return (
			this.myEntry === null &&
			this.$root.$data.loggedIn &&
			this.queue.isOpen(this.time) &&
			isValidDescription &&
			(this.location.trim() !== '' || !this.queue.config?.enableLocationField)
		);
	}

	get myEntryIndex(): number {
		return this.queue.entryIndex(this.$root.$data.userInfo.email);
	}

	get myEntry(): QueueEntry | null {
		return this.queue.entry(this.$root.$data.userInfo.email);
	}

	get myEntryModified() {
		const e = this.myEntry;
		if (e === null) return false;

		if (this.hasDescWithPrompts()) {
			try {
				const parsedDescription = JSON.parse(e.description || '{}');
				return (
					JSON.stringify(parsedDescription) !==
						this.descWithPromptsToDescription() || e.location !== this.location
				);
			} catch (e) {
				// If description is not valid JSON, consider it modified
				return true;
			}
		}

		return e.description !== this.description || e.location !== this.location;
	}

	signUp() {
		if (this.queue.config?.confirmSignupMessage !== undefined) {
			return this.$buefy.dialog.confirm({
				title: 'Sign Up',
				message: EscapeHTML(this.queue.config!.confirmSignupMessage),
				type: 'is-warning',
				hasIcon: true,
				onConfirm: this.signUpRequest,
			});
		}

		this.signUpRequest();
	}

	signUpRequest() {
		// No, this doesn't prevent students from manually hitting the API to specify
		// a location. l33t h4x!
		const location = this.queue.config?.enableLocationField
			? this.location
			: '(disabled)';

		const description = this.hasDescWithPrompts()
			? this.descWithPromptsToDescription()
			: this.description;

		fetch(process.env.BASE_URL + `api/queues/${this.queue.id}/entries`, {
			method: 'POST',
			body: JSON.stringify({
				description,
				location,
			}),
		}).then((res) => {
			if (res.status !== 201) {
				return ErrorDialog(res);
			}

			this.$buefy.toast.open({
				duration: 5000,
				message: `You're on the queue, ${EscapeHTML(
					this.$root.$data.userInfo.first_name
				)}!`,
				type: 'is-success',
			});
		});
	}

	updateRequest() {
		const description = this.hasDescWithPrompts()
			? this.descWithPromptsToDescription()
			: this.description;

		if (this.myEntry !== null) {
			fetch(
				process.env.BASE_URL +
					`api/queues/${this.queue.id}/entries/${this.myEntry.id}`,
				{
					method: 'PUT',
					body: JSON.stringify({
						description,
						location: this.location,
					}),
				}
			).then((res) => {
				if (res.status !== 204) {
					return ErrorDialog(res);
				}

				this.$buefy.toast.open({
					duration: 5000,
					message: 'Your request has been updated!',
					type: 'is-success',
				});
			});
		}
	}
}
</script>
