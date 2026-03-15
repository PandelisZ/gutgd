export function formatPayload(payload) {
  if (payload == null) {
    return ''
  }
  if (typeof payload === 'string') {
    return payload
  }
  return JSON.stringify(payload, null, 2)
}
