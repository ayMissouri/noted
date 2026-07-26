<script lang="ts">
  import { api, ApiError, clearToken, hasToken, type Note, type NoteListItem } from './lib/api'
  import Devices from './lib/Devices.svelte'
  import Editor from './lib/Editor.svelte'
  import Login from './lib/Login.svelte'
  import Setup from './lib/Setup.svelte'

  let screen = $state<'loading' | 'setup' | 'login' | 'app'>('loading')
  let view = $state<'notes' | 'settings'>('notes')
  let vaultId = $state<string | null>(null)
  let vaultName = $state('')
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
      const vaults = await api.vaults()
      vaultId = vaults[0].id
      vaultName = vaults[0].name
      notes = await api.notes(vaultId)
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
    <p class="vault">{vaultName}</p>
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
    {:else}
      <p class="empty">Pick a note, or create one.</p>
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
  .vault {
    margin: 0 0 1rem;
    color: var(--muted);
    font-size: 0.85rem;
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
