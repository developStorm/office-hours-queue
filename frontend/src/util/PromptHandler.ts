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
 * Parse a description string into an array of prompt responses
 * @param description The JSON description string
 * @param prompts Array of prompts to match against
 */
export function descriptionToResponses(
	description?: string,
	prompts?: string[]
): string[] {
	if (!description) return [];
	if (!prompts || !prompts.length) return [];

	try {
		const parsedData = JSON.parse(description || '[]');

		// Handle array format (preferred)
		if (Array.isArray(parsedData)) {
			// Validate each item is a string
			for (const item of parsedData) {
				if (typeof item !== 'string') {
					return prompts.map(() => '');
				}
			}
			return parsedData;
		}

		// For backward compatibility - handle old object format
		// by extracting values in order of prompts
		if (typeof parsedData === 'object' && parsedData !== null) {
			for (const key in parsedData) {
				if (typeof key !== 'string' || typeof parsedData[key] !== 'string') {
					return prompts.map(() => '');
				}
			}

			return Object.values(parsedData);
		}

		// Not valid JSON array or object
		return prompts.map(() => '');
	} catch (e) {
		return prompts.map(() => '');
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
