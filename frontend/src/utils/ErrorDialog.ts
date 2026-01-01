import { globalDialog } from '@/composables/useDialog'
import { escapeHTML } from '@/utils/sanitization'

function showErrorDialog(message: string): void {
  globalDialog.alert({
    title: 'Request Failed',
    message: message,
    type: 'danger',
    hasIcon: true,
  })
}

export default async function ErrorDialog(res: Response): Promise<void> {
  try {
    const data = await res.json()
    showErrorDialog(escapeHTML(data.message))
  } catch {
    showErrorDialog(
      `An unknown error occurred while fetching endpoint <code>${escapeHTML(
        new URL(res.url).pathname
      )}</code>. <a href="https://developer.mozilla.org/docs/Web/HTTP/Status/${
        res.status
      }" target="_blank">HTTP Status: ${res.status}</a>`
    )
  }
}
