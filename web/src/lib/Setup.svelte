<script lang="ts">
  import { api, ApiError } from './api'

  let { oncomplete }: { oncomplete: () => void } = $props()

  let username = $state('')
  let email = $state('')
  let password = $state('')
  let confirm = $state('')
  let busy = $state(false)
  let error = $state('')

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    if (password !== confirm) {
      error = 'The passwords do not match.'
      return
    }
    busy = true
    error = ''
    try {
      await api.setup(username.trim(), email.trim(), password)
      oncomplete()
    } catch (err) {
      error = err instanceof ApiError ? err.message : 'Could not reach the server.'
    } finally {
      busy = false
    }
  }
</script>

<main class="setup">
  <h1>noted</h1>
  <p>Welcome. Create the admin account for this server. It manages every other account.</p>
  <form onsubmit={submit}>
    <label>
      Username
      <input bind:value={username} autocomplete="username" required />
    </label>
    <label>
      Email <span class="optional">(optional)</span>
      <input type="email" bind:value={email} autocomplete="email" />
    </label>
    <label>
      Password
      <input type="password" bind:value={password} autocomplete="new-password" required minlength="8" />
    </label>
    <label>
      Confirm password
      <input type="password" bind:value={confirm} autocomplete="new-password" required />
    </label>
    {#if error}
      <p class="error">{error}</p>
    {/if}
    <button disabled={busy}>{busy ? 'Creating…' : 'Create admin account'}</button>
  </form>
</main>

<style>
  .setup {
    max-width: 22rem;
    margin: 4rem auto;
    padding: 0 1rem;
  }
  h1 {
    margin-bottom: 0.25rem;
  }
  form {
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
    margin-top: 1.5rem;
  }
  label {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    font-size: 0.9rem;
  }
  .optional {
    color: var(--muted);
  }
  input {
    padding: 0.45rem 0.6rem;
  }
  button {
    padding: 0.5rem;
    margin-top: 0.5rem;
  }
  .error {
    margin: 0;
    color: var(--err);
  }
</style>
