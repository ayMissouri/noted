<script lang="ts">
  let status = $state<'checking' | 'online' | 'offline'>('checking')

  async function check() {
    try {
      const res = await fetch('/healthz')
      status = res.ok ? 'online' : 'offline'
    } catch {
      status = 'offline'
    }
  }
  check()
</script>

<main>
  <h1>noted</h1>
  <p class={status}>server {status}</p>
</main>

<style>
  main {
    max-width: 40rem;
    margin: 4rem auto;
    padding: 0 1rem;
  }
  .online {
    color: var(--ok);
  }
  .offline {
    color: var(--err);
  }
</style>
