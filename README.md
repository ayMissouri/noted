## Quickstart

[Go](https://go.dev) 1.26+ required.

```bash
git clone https://github.com/ayMissouri/noted && cd noted
make dev # go run ./cmd/noted
```

Then open http://localhost:6683.

With Docker (includes a Caddy reverse proxy on port 80):

```
docker compose up -d --build
```

## Licence

[MIT](LICENSE).
