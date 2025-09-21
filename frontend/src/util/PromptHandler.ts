/**
 * Utility functions for handling queue entry prompts
 */

/**
 * Check if a queue has custom prompts defined
 * @param queueConfig Queue configuration object
 */
export function hasCustomPrompts(queueConfig: any): boolean {
	return Boolean(queueConfig?.prompts?.length);
}

/**
 * Convert an array of responses to a JSON string
 * @param responses Array of responses to prompts
 */
export function responsesToDescription(responses: string[]): string {
	return JSON.stringify(responses);
}

/**
 * Parse a description string into an array of prompt responses.
 * Behavior:
 * - If description is a JSON array of strings, return the array.
 * - If description is a JSON object whose values are strings, return Object.values in key order.
 * - If parsing fails or structure is invalid, return an empty array (treat as non-prompted description).
 *
 * @param description The raw description string stored on the entry
 */
export function descriptionToResponses(description?: string): string[] {
	if (!description) return [];

	try {
		const parsedData = JSON.parse(description);

		// Preferred format: array of strings
		if (Array.isArray(parsedData)) {
			for (const item of parsedData) {
				if (typeof item !== 'string') return [];
			}
			return parsedData;
		}

		// Legacy format: object with string values
		if (typeof parsedData === 'object' && parsedData !== null) {
			const values: string[] = [];
			for (const key in parsedData as Record<string, unknown>) {
				const val = (parsedData as Record<string, unknown>)[key];
				if (typeof key !== 'string' || typeof val !== 'string') return [];
				values.push(val);
			}
			return values;
		}

		return [];
	} catch {
		// Not JSON: treat as plain description (non-prompted)
		return [];
	}
}

/**
 * Check if a prompt response array has all required fields filled
 * @param responses Array of responses to check
 */
export function areResponsesValid(
	responses: string[],
	prompts: string[]
): boolean {
	return (
		responses.length === prompts.length &&
		responses.every((r) => r.trim() !== '')
	);
}

/**
 * Calculate the total character length of the JSON-encoded responses
 * @param responses Array of responses
 */
export function getResponsesLength(responses: string[]): number {
	if (!responses.length) return 0;
	return responsesToDescription(responses).length;
}
