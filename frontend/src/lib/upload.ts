async function digestHex(input: ArrayBuffer): Promise<string> {
  const hash = await crypto.subtle.digest('SHA-256', input);
  return Array.from(new Uint8Array(hash))
    .map((value) => value.toString(16).padStart(2, '0'))
    .join('');
}

export async function sha256ForFile(file: File): Promise<string> {
  const buffer = await file.arrayBuffer();
  return digestHex(buffer);
}

export async function sha256ForBytes(bytes: Uint8Array): Promise<string> {
  const slice = bytes.byteOffset === 0 && bytes.byteLength === bytes.buffer.byteLength
    ? bytes.buffer
    : bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength);
  return digestHex(slice);
}

export async function buildBrowserFingerprint(extra: string[] = []): Promise<string> {
  const parts = [
    navigator.userAgent,
    navigator.language,
    navigator.platform,
    Intl.DateTimeFormat().resolvedOptions().timeZone,
    String(navigator.hardwareConcurrency || 0),
    `${window.screen.width}x${window.screen.height}`,
    ...extra,
  ];

  const data = new TextEncoder().encode(parts.join('|'));
  return digestHex(data.buffer);
}

export function formatBytes(totalSize: number): string {
  if (totalSize >= 1024 * 1024 * 1024) {
    return `${(totalSize / (1024 * 1024 * 1024)).toFixed(1)} GB`;
  }
  if (totalSize >= 1024 * 1024) {
    return `${(totalSize / (1024 * 1024)).toFixed(1)} MB`;
  }
  if (totalSize >= 1024) {
    return `${(totalSize / 1024).toFixed(1)} KB`;
  }
  return `${totalSize} B`;
}
