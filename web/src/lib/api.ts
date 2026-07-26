export interface Vault {
  id: string
  name: string
  created_at: string
  updated_at: string
  change_seq: number
}

export interface NoteListItem {
  id: string
  vault_id: string
  name: string
  version: number
  created_at: string
  updated_at: string
  change_seq: number
}

export interface Note extends NoteListItem {
  body: string
}

export interface User {
  id: string
  username: string
  email: string | null
  is_admin: boolean
  created_at: string
}

export interface Device {
  id: string
  name: string
  kind: string
  created_at: string
  last_seen_at: string | null
  current: boolean
}

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
  ) {
    super(message)
  }
}

const TOKEN_KEY = 'noted.token'
let token: string | null = localStorage.getItem(TOKEN_KEY)

export function hasToken(): boolean {
  return token !== null
}

export function clearToken(): void {
  token = null
  localStorage.removeItem(TOKEN_KEY)
}

function storeToken(t: string): void {
  token = t
  localStorage.setItem(TOKEN_KEY, t)
}

function deviceName(): string {
  const ua = navigator.userAgent
  const browser = ua.includes('Firefox')
    ? 'Firefox'
    : ua.includes('Edg')
      ? 'Edge'
      : ua.includes('Chrome')
        ? 'Chrome'
        : ua.includes('Safari')
          ? 'Safari'
          : 'Browser'
  const os = ua.includes('Windows')
    ? 'Windows'
    : ua.includes('Mac')
      ? 'macOS'
      : ua.includes('Android')
        ? 'Android'
        : ua.includes('iPhone') || ua.includes('iPad')
          ? 'iOS'
          : ua.includes('Linux')
            ? 'Linux'
            : ''
  return os ? `${browser} on ${os}` : browser
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let res: Response
  try {
    res = await fetch(path, {
      headers: {
        'Content-Type': 'application/json',
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      },
      ...init,
    })
  } catch {
    throw new ApiError(0, 'network', 'cannot reach the server')
  }
  if (!res.ok) {
    let code = 'unknown'
    let message = res.statusText
    try {
      const body = await res.json()
      code = body.error.code
      message = body.error.message
    } catch {}
    throw new ApiError(res.status, code, message)
  }
  if (res.status === 204) {
    return undefined as T
  }
  return res.json() as Promise<T>
}

export const api = {
  setupStatus: () =>
    request<{ needs_setup: boolean }>('/api/v1/setup').then((r) => r.needs_setup),

  setup: (username: string, email: string, password: string) =>
    request<User>('/api/v1/setup', {
      method: 'POST',
      body: JSON.stringify({ username, email, password }),
    }),

  login: (username: string, password: string) =>
    request<{ token: string; user: User }>('/api/v1/login', {
      method: 'POST',
      body: JSON.stringify({ username, password, device_name: deviceName() }),
    }).then((r) => {
      storeToken(r.token)
      return r.user
    }),

  devices: () => request<{ devices: Device[] }>('/api/v1/devices').then((r) => r.devices),

  revokeDevice: (id: string) =>
    request<void>(`/api/v1/devices/${encodeURIComponent(id)}`, { method: 'DELETE' }),

  vaults: () => request<{ vaults: Vault[] }>('/api/v1/vaults').then((r) => r.vaults),

  createVault: (name: string) =>
    request<Vault>('/api/v1/vaults', { method: 'POST', body: JSON.stringify({ name }) }),

  renameVault: (id: string, name: string) =>
    request<Vault>(`/api/v1/vaults/${encodeURIComponent(id)}`, {
      method: 'PATCH',
      body: JSON.stringify({ name }),
    }),

  deleteVault: (id: string) =>
    request<void>(`/api/v1/vaults/${encodeURIComponent(id)}`, { method: 'DELETE' }),

  notes: (vaultId: string) =>
    request<{ notes: NoteListItem[] }>(
      `/api/v1/vaults/${encodeURIComponent(vaultId)}/notes`,
    ).then((r) => r.notes),

  note: (id: string) => request<Note>(`/api/v1/notes/${encodeURIComponent(id)}`),

  createNote: (vaultId: string, name: string, body: string) =>
    request<Note>(`/api/v1/vaults/${encodeURIComponent(vaultId)}/notes`, {
      method: 'POST',
      body: JSON.stringify({ name, body }),
    }),

  updateNote: (id: string, body: string, baseVersion: number) =>
    request<Note>(`/api/v1/notes/${encodeURIComponent(id)}`, {
      method: 'PUT',
      body: JSON.stringify({ body, base_version: baseVersion }),
    }),

  render: (markdown: string) =>
    request<{ html: string }>('/api/v1/render', {
      method: 'POST',
      body: JSON.stringify({ markdown }),
    }).then((r) => r.html),
}
