export default defineEventHandler((event) => {
  const config = useRuntimeConfig(event)
  return proxyRequest(event, `${String(config.apiBase).replace(/\/$/, '')}${event.path}`)
})
