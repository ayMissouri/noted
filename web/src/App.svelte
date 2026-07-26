<script lang="ts">
  import { api, ApiError, clearToken, hasToken, type Note, type NoteListItem, type Vault } from './lib/api'
  import Devices from './lib/Devices.svelte'
  import Editor from './lib/Editor.svelte'
  import Login from './lib/Login.svelte'
  import Setup from './lib/Setup.svelte'

  let screen = $state<'loading' | 'setup' | 'login' | 'app'>('loading')
  let view = $state<'notes' | 'settings'>('notes')
  let vaults = $state<Vault[]>([])
  let vaultId = $state<string | null>(null)
  let notes = $state<NoteListItem[]>([])
  let current = $state<Note | null>(null)
  let newName = $state('')
  let error = $state('')

  async function boot() {
    try {
      if (await api.setupStatus()) {
        screen = 'setup'
        return
      }
    } catch (e) {
      error = e instanceof Error ? e.message : String(e)
    }
    if (!hasToken()) {
      screen = 'login'
      return
    }
    await enter()
  }

  async function enter() {
    try {
      vaults = await api.vaults()
      if (vaults.length > 0) {
        await switchVault(vaults[0].id)
      } else {
        vaultId = null
        notes = []
      }
      error = ''
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) {
        clearToken()
        screen = 'login'
        return
      }
      error = e instanceof Error ? e.message : String(e)
    }
    screen = 'app'
  }
  boot()

  async function switchVault(id: string) {
    vaultId = id
    current = null
    notes = await api.notes(id)
  }

  async function onVaultChange() {
    if (!vaultId) return
    try {
      await switchVault(vaultId)
      error = ''
    } catch (e) {
      error = e instanceof Error ? e.message : String(e)
    }
  }

  async function createVaultUI() {
    const name = window.prompt('Name for the new vault:')
    if (!name?.trim()) return
    try {
      const vault = await api.createVault(name.trim())
      vaults = await api.vaults()
      await switchVault(vault.id)
      error = ''
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'could not create the vault'
    }
  }

  async function renameVaultUI() {
    if (!vaultId) return
    const currentName = vaults.find((v) => v.id === vaultId)?.name ?? ''
    const name = window.prompt('Rename vault:', currentName)
    if (!name?.trim() || name.trim() === currentName) return
    try {
      await api.renameVault(vaultId, name.trim())
      vaults = await api.vaults()
      error = ''
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'could not rename the vault'
    }
  }

  async function deleteVaultUI() {
    if (!vaultId) return
    const name = vaults.find((v) => v.id === vaultId)?.name ?? 'this vault'
    if (!window.confirm(`Delete the vault "${name}"? Its notes become inaccessible.`)) return
    try {
      await api.deleteVault(vaultId)
      vaults = await api.vaults()
      if (vaults.length > 0) {
        await switchVault(vaults[0].id)
      } else {
        vaultId = null
        notes = []
        current = null
      }
      error = ''
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'could not delete the vault'
    }
  }

  function logout() {
    clearToken()
    notes = []
    current = null
    vaultId = null
    view = 'notes'
    screen = 'login'
  }

  function openNote(id: string) {
    view = 'notes'
    open(id)
  }

  async function open(id: string) {
    try {
      current = await api.note(id)
      error = ''
    } catch (e) {
      error = e instanceof Error ? e.message : String(e)
    }
  }

  async function createNote(e: SubmitEvent) {
    e.preventDefault()
    const name = newName.trim()
    if (!vaultId || !name) return
    try {
      const note = await api.createNote(vaultId, name, '')
      newName = ''
      notes = await api.notes(vaultId)
      current = note
      error = ''
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'could not create the note'
    }
  }

  function onSaved(saved: Note) {
    notes = notes.map((n) =>
      n.id === saved.id ? { ...n, version: saved.version, updated_at: saved.updated_at } : n,
    )
    if (current && current.id === saved.id) {
      current = saved
    }
  }
</script>

{#if screen === 'setup'}
  <Setup oncomplete={enter} />
{:else if screen === 'login'}
  <Login oncomplete={enter} />
{:else if screen === 'app'}
<div class="layout">
  <aside>
    <h1>noted</h1>
    <div class="vaultbar">
      <select bind:value={vaultId} onchange={onVaultChange} aria-label="Vault">
        {#each vaults as v (v.id)}
          <option value={v.id}>{v.name}</option>
        {/each}
      </select>
      <button class="ghost" title="New vault" onclick={createVaultUI}>+</button>
    </div>
    <div class="vaultactions">
      <button class="ghost" onclick={renameVaultUI} disabled={!vaultId}>Rename</button>
      <button class="ghost" onclick={deleteVaultUI} disabled={!vaultId}>Delete</button>
    </div>
    <form onsubmit={createNote}>
      <input placeholder="New note name" bind:value={newName} />
      <button type="submit">Add</button>
    </form>
    <nav>
      {#each notes as n (n.id)}
        <button class="note" class:active={view === 'notes' && current?.id === n.id} onclick={() => openNote(n.id)}>
          {n.name}
        </button>
      {/each}
      {#if notes.length === 0}
        <p class="empty">No notes yet.</p>
      {/if}
    </nav>
    <div class="footer">
      <button class="ghost" onclick={() => (view = view === 'settings' ? 'notes' : 'settings')}>
        {view === 'settings' ? 'Back to notes' : 'Settings'}
      </button>
      <button class="ghost" onclick={logout}>Log out</button>
    </div>
  </aside>

  <main>
    {#if error}
      <p class="error">{error}</p>
    {/if}
    {#if view === 'settings'}
      <Devices oncurrentrevoked={logout} />
    {:else if current}
      <Editor note={current} onsaved={onSaved} />
    {:else if vaultId}
      <p class="empty">Pick a note, or create one.</p>
    {:else}
      <p class="empty">No vaults. Create one with the + button.</p>
    {/if}
  </main>
</div>
{/if}

<style>
  .layout {
    display: grid;
    grid-template-columns: 16rem 1fr;
    height: 100vh;
  }
  aside {
    border-right: 1px solid var(--border);
    padding: 1rem;
    overflow-y: auto;
  }
  h1 {
    margin: 0 0 0.25rem;
    font-size: 1.3rem;
  }
  .vaultbar {
    display: flex;
    gap: 0.4rem;
    margin-bottom: 0.4rem;
  }
  .vaultbar select {
    flex: 1;
    min-width: 0;
    font: inherit;
  }
  .vaultactions {
    display: flex;
    gap: 0.4rem;
    margin-bottom: 1rem;
  }
  .vaultactions .ghost {
    font-size: 0.8rem;
    padding: 0.2rem 0.5rem;
  }
  form {
    display: flex;
    gap: 0.5rem;
    margin-bottom: 1rem;
  }
  input {
    flex: 1;
    min-width: 0;
  }
  nav {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }
  .note {
    text-align: left;
    background: none;
    border: none;
    padding: 0.4rem 0.5rem;
    border-radius: 0.3rem;
    color: inherit;
    cursor: pointer;
    font-size: 0.95rem;
  }
  .note:hover {
    background: var(--hover);
  }
  .note.active {
    background: var(--active);
  }
  .footer {
    display: flex;
    gap: 0.5rem;
    margin-top: 1.5rem;
  }
  .ghost {
    background: none;
    border: 1px solid var(--border);
    border-radius: 0.3rem;
    padding: 0.35rem 0.6rem;
    color: var(--muted);
    cursor: pointer;
  }
  main {
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .error {
    margin: 0;
    padding: 0.5rem 1rem;
    color: var(--err);
  }
  .empty {
    color: var(--muted);
    padding: 1rem;
  }
</style>
