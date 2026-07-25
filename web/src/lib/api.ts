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

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
  ) {
    super(message)
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let res: Response
  try {
    res = await fetch(path, {
      headers: { 'Content-Type': 'application/json' },
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
  return res.json() as Promise<T>
}

export const api = {
  vaults: () => request<{ vaults: Vault[] }>('/api/v1/vaults').then((r) => r.vaults),

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
