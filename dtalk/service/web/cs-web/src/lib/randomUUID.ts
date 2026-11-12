/**
 * 生成 RFC 4122 v4 UUID。
 *
 * 浏览器里 crypto.randomUUID() 仅在「安全上下文」可用（HTTPS、localhost 等）；
 * 通过 http://公网IP 或 http://局域网IP 访问时不可用，需用 getRandomValues 回退。
 */
export function randomUUID(): string {
  const c = typeof globalThis !== 'undefined' ? globalThis.crypto : undefined;
  if (c && typeof c.randomUUID === 'function') {
    return c.randomUUID();
  }
  if (!c || typeof c.getRandomValues !== 'function') {
    console.error('[randomUUID] Web Crypto 不可用：缺少 randomUUID 与 getRandomValues');
    throw new Error('Web Crypto API unavailable (randomUUID / getRandomValues)');
  }
  const bytes = new Uint8Array(16);
  c.getRandomValues(bytes);
  bytes[6] = (bytes[6]! & 0x0f) | 0x40;
  bytes[8] = (bytes[8]! & 0x3f) | 0x80;
  const hex = [...bytes].map((b) => b.toString(16).padStart(2, '0')).join('');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}
