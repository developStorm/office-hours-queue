import Queue from '@/types/Queue';
import Course from '@/types/Course';

/**
 * Determine whether the sidebar should be shown based on current state
 *
 * @param context Vue component context with access to $root data
 * @param queueOrID Optional queue object for current page
 * @returns True if the sidebar should be shown, false otherwise
 */
export function shouldShowCourses(
	context: any,
	queueOrID?: string | Queue
): boolean {
	if (!queueOrID) return true; // Non-queue pages should always show the sidebar
	if (window.innerWidth < 1024) return false;

	// Get the queue from the store
	const queue =
		typeof queueOrID === 'string'
			? context.$root.$data.queues.find((q: Queue) => q.id === queueOrID)
			: queueOrID;

	// Check if user is admin for this queue's course
	const isCourseAdmin =
		!context.$root.$data.studentView &&
		context.$root.$data.loggedIn &&
		context.$root.$data.userInfo.admin_courses !== undefined &&
		context.$root.$data.userInfo.admin_courses.includes(queue.course.id);

	// Check if course has multiple queues
	const numQueuesOfCourse = Object.values(
		context.$root.$data.courses as Course[]
	).find((c) => c.id === queue.course.id)?.queues.length;
	const hasMultipleQueues =
		numQueuesOfCourse !== undefined && numQueuesOfCourse > 1;

	// On desktop queue pages, show sidebar if user is logged in, admin, and course has multiple queues
	return isCourseAdmin && hasMultipleQueues;
}
