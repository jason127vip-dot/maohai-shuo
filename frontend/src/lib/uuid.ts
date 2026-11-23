/** Request/task identifiers, including browsers visiting a public HTTP test address. */
export function createRandomId(): string {
  const cryptoApi = globalThis.crypto
  if (typeof cryptoApi?.randomUUID === 'function') return cryptoApi.randomUUID()

  // getRandomValues is available on HTTP too; never replace it with Math.random.
  if (typeof cryptoApi?.getRandomValues !== 'function') {
    throw new Error('当前浏览器不支持安全随机编号，请使用新版浏览器或 HTTPS 地址访问。')
  }
  const bytes = cryptoApi.getRandomValues(new Uint8Array(16))
  bytes[6] = (bytes[6] & 0x0f) | 0x40
  bytes[8] = (bytes[8] & 0x3f) | 0x80
  const hex = Array.from(bytes, byte => byte.toString(16).padStart(2, '0')).join('')
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
}
