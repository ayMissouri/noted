<script lang="ts">
  import { api, ApiError, type Device } from './api'

  let { oncurrentrevoked }: { oncurrentrevoked: () => void } = $props()

  let devices = $state<Device[]>([])
  let error = $state('')

  async function refresh() {
    try {
      devices = await api.devices()
      error = ''
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Could not load devices.'
    }
  }
  refresh()

  async function revoke(d: Device) {
    try {
      await api.revokeDevice(d.id)
      if (d.current) {
        oncurrentrevoked()
        return
      }
      await refresh()
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Could not revoke the device.'
    }
  }

  function when(iso: string | null): string {
    return iso ? new Date(iso).toLocaleString() : 'never'
  }
</script>

<section class="devices">
  <h2>Devices</h2>
  <p class="hint">Every signed-in device and API token for your account. Revoking one signs it out on its next request.</p>
  {#if error}
    <p class="error">{error}</p>
  {/if}
  <ul>
    {#each devices as d (d.id)}
      <li>
        <div>
          <strong>{d.name}</strong>
          <span class="kind">{d.kind}</span>
          {#if d.current}<span class="current">this device</span>{/if}
          <br />
          <small>added {when(d.created_at)} | last seen {when(d.last_seen_at)}</small>
        </div>
        <button onclick={() => revoke(d)}>{d.current ? 'Revoke and log out' : 'Revoke'}</button>
      </li>
    {/each}
  </ul>
</section>

<style>
  .devices {
    padding: 1.5rem;
    max-width: 36rem;
  }
  h2 {
    margin-top: 0;
  }
  .hint {
    color: var(--muted);
    font-size: 0.9rem;
  }
  ul {
    list-style: none;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }
  li {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
    border: 1px solid var(--border);
    border-radius: 0.4rem;
    padding: 0.6rem 0.9rem;
  }
  .kind {
    color: var(--muted);
    font-size: 0.8rem;
    text-transform: uppercase;
    margin-left: 0.4rem;
  }
  .current {
    color: var(--ok);
    font-size: 0.8rem;
    margin-left: 0.4rem;
  }
  small {
    color: var(--muted);
  }
  .error {
    color: var(--err);
  }
</style>
