<script lang="ts">
  import { api, ApiError } from './api'

  let { oncomplete }: { oncomplete: () => void } = $props()

  let username = $state('')
  let password = $state('')
  let busy = $state(false)
  let error = $state('')

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    error = ''
    try {
      await api.login(username.trim(), password)
      oncomplete()
    } catch (err) {
      error = err instanceof ApiError ? err.message : 'Could not reach the server.'
    } finally {
      busy = false
    }
  }
</script>

<main class="login">
  <h1>noted</h1>
  <form onsubmit={submit}>
    <label>
      Username
      <input bind:value={username} autocomplete="username" required />
    </label>
    <label>
      Password
      <input type="password" bind:value={password} autocomplete="current-password" required />
    </label>
    {#if error}
      <p class="error">{error}</p>
    {/if}
    <button disabled={busy}>{busy ? 'Signing in…' : 'Sign in'}</button>
  </form>
</main>

<style>
  .login {
    max-width: 22rem;
    margin: 4rem auto;
    padding: 0 1rem;
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
