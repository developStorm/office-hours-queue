import type { ApiResponse } from '@/types'

export function useApi() {
  async function request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<ApiResponse<T>> {
    try {
      const res = await fetch(endpoint, {
        ...options,
        headers: {
          'Content-Type': 'application/json',
          ...options.headers,
        },
      })

      if (!res.ok) {
        let error = 'Unknown error'
        try {
          const data = await res.json()
          error = data.message || data.error || error
        } catch {
          // Ignore JSON parse error
        }
        return { status: res.status, error }
      }

      // Handle 204 No Content
      if (res.status === 204) {
        return { status: res.status, data: undefined }
      }

      const data = await res.json()
      return { status: res.status, data }
    } catch (e) {
      return { status: 0, error: String(e) }
    }
  }

  return {
    get: <T>(url: string) => request<T>(url),

    post: <T>(url: string, body?: unknown) =>
      request<T>(url, {
        method: 'POST',
        body: body ? JSON.stringify(body) : undefined,
      }),

    put: <T>(url: string, body?: unknown) =>
      request<T>(url, {
        method: 'PUT',
        body: body ? JSON.stringify(body) : undefined,
      }),

    delete: <T>(url: string) =>
      request<T>(url, { method: 'DELETE' }),
  }
}
