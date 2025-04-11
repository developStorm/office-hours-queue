import { DialogProgrammatic as Dialog } from 'buefy';
import EscapeHTML from '@/util/Sanitization';

function showErrorDialog(message: string): void {
	Dialog.alert({
		title: 'Request Failed',
		message: message,
		type: 'is-danger',
		hasIcon: true,
	});
}

export default async function ErrorDialog(res: Response): Promise<any> {
	return res
		.json()
		.then((data) => {
			showErrorDialog(EscapeHTML(data.message));
		})
		.catch(() => {
			showErrorDialog(
				`An unknown error occurred while fetching endpoint <code>${EscapeHTML(
					new URL(res.url).pathname
				)}</code>. <a href="https://developer.mozilla.org/docs/Web/HTTP/Status/${
					res.status
				}" target="_blank">HTTP Status: ${res.status}</a>`
			);
		});
}
