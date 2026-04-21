import { useEffect, useRef } from 'react'

/**
 * Subscribe to Server-Sent Events from /api/events.
 * Calls `onChange` whenever the backend detects new transactions.
 */
export function useSSE(onChange: () => void) {
  const onChangeRef = useRef(onChange)
  onChangeRef.current = onChange

  useEffect(() => {
    let es: EventSource | null = null
    let retryTimer: ReturnType<typeof setTimeout> | null = null

    function connect() {
      es = new EventSource('/api/events')

      es.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data)
          if (data.type === 'change') {
            onChangeRef.current()
          }
        } catch {
          // ignore parse errors
        }
      }

      es.onerror = () => {
        es?.close()
        // Reconnect after 3 seconds
        retryTimer = setTimeout(connect, 3000)
      }
    }

    connect()

    return () => {
      es?.close()
      if (retryTimer) clearTimeout(retryTimer)
    }
  }, [])
}
