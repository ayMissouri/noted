<script lang="ts">
  import { api, ApiError, type Note } from './api'

  let { note, onsaved }: { note: Note; onsaved: (n: Note) => void } = $props()

  let body = $state('')
  let version = $state(0)
  let status = $state<'saved' | 'unsaved' | 'saving' | 'conflict' | 'error'>('saved')
  let errorMsg = $state('')
  let html = $state('')

  $effect(() => {
    body = note.body
    version = note.version
    status = 'saved'
    errorMsg = ''
  })

  let timer: ReturnType<typeof setTimeout> | undefined
  $effect(() => {
    const text = body
    clearTimeout(timer)
    timer = setTimeout(async () => {
      try {
        html = await api.render(text)
      } catch {}
    }, 300)
    return () => clearTimeout(timer)
  })

  function edited() {
    if (status !== 'conflict') status = 'unsaved'
  }

  async function save() {
    if (status === 'saving' || status === 'saved') return
    status = 'saving'
    try {
      const updated = await api.updateNote(note.id, body, version)
      version = updated.version
      status = 'saved'
      onsaved(updated)
    } catch (e) {
      if (e instanceof ApiError && e.code === 'version_conflict') {
        status = 'conflict'
        errorMsg = 'This note changed elsewhere since you opened it.'
      } else {
        status = 'error'
        errorMsg = e instanceof Error ? e.message : String(e)
      }
    }
  }

  async function reload() {
    const fresh = await api.note(note.id)
    body = fresh.body
    version = fresh.version
    status = 'saved'
    errorMsg = ''
    onsaved(fresh)
  }

  function onkeydown(e: KeyboardEvent) {
    if ((e.ctrlKey || e.metaKey) && e.key === 's') {
      e.preventDefault()
      save()
    }
  }
</script>

<svelte:window onkeydown={onkeydown} />

<section class="editor">
  <header>
    <h2>{note.name}</h2>
    <span class="status {status}">{status}</span>
    <button onclick={save} disabled={status === 'saving' || status === 'saved'}>Save</button>
  </header>

  {#if errorMsg}
    <p class="banner {status}">
      {errorMsg}
      {#if status === 'conflict'}
        <button onclick={reload}>Reload note (discards local edits)</button>
      {/if}
    </p>
  {/if}

  <div class="panes">
    <textarea bind:value={body} oninput={edited} spellcheck="false"></textarea>
    <div class="preview">{@html html}</div>
  </div>
</section>

<style>
  .editor {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-width: 0;
  }
  header {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.5rem 1rem;
    border-bottom: 1px solid var(--border);
  }
  h2 {
    margin: 0;
    font-size: 1.1rem;
    flex: 1;
  }
  .status {
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .status.saved { color: var(--ok); }
  .status.unsaved { color: var(--muted); }
  .status.saving { color: var(--muted); }
  .status.conflict, .status.error { color: var(--err); }
  .banner {
    margin: 0;
    padding: 0.5rem 1rem;
    background: color-mix(in srgb, var(--err) 12%, transparent);
  }
  .panes {
    display: grid;
    grid-template-columns: 1fr 1fr;
    flex: 1;
    min-height: 0;
  }
  textarea {
    resize: none;
    border: none;
    border-right: 1px solid var(--border);
    padding: 1rem;
    font: 0.95rem/1.6 ui-monospace, monospace;
    background: transparent;
    color: inherit;
  }
  textarea:focus {
    outline: none;
  }
  .preview {
    padding: 1rem;
    overflow-y: auto;
  }
</style>
