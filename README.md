# civil-os

civil-os is an open-source platform for parcel and property data. It stores
parcels, their geometry, attributes, improvements, zoning, sales and valuations
in a bitemporal PostGIS database. It serves them through an authenticated API
and vector map tiles.

One deployment serves one jurisdiction. Every service is a container, and the
whole stack runs locally with Docker Compose on synthetic data.

- **[Architecture](docs/architecture.md):** services, APIs, data model,
  configuration and releases.
- **[Public API](api/proto/public/README.md):** the services and how to call
  them, plus the TypeScript SDK
  [`@civil-labs/civil-os-api`](sdk/js/README.md).

## Quickstart

You need [Docker](https://docs.docker.com/get-docker/) with Compose v2.

```sh
git clone https://github.com/civil-labs/civil-os.git
cd civil-os
docker compose up --build --wait
```

This starts the following, all on fictional data in open ocean:

- PostGIS, with every migration applied and a synthetic dataset of 20 parcels;
- a local [Dex](https://dexidp.io) OIDC server;
- the gateway on <http://localhost:8080>, with the reader, writer and tile
  server behind it.

`docker compose down -v` stops everything and deletes the database.

### Try the API with curl

Get an ID token for the local test user. These are throwaway credentials from
`dev/dex.yaml`:

```sh
TOKEN=$(curl -s http://localhost:5556/dex/token \
  -d grant_type=password -d scope='openid email profile' \
  -d username=dev@example.com -d password=civil-os-dev-password \
  -d client_id=civil-os-dev -d client_secret=civil-os-dev-secret \
  | sed -E 's/.*"id_token":"([^"]+)".*/\1/')
```

Read a parcel. The API speaks [Connect](https://connectrpc.com), so a JSON
`POST` to `/<package>.<Service>/<Method>` works:

```sh
curl -s http://localhost:8080/civil.public.parcels.v1.ParcelsService/GetParcelsWithImprovementSummaryByParcelId \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"parcelIds": ["a0000000-0000-4000-8000-000000000002"]}'
```

Update its frontage, then read it back:

```sh
curl -s http://localhost:8080/civil.public.parcels.v1.ParcelsService/UpdateParcel \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"parcelId": "a0000000-0000-4000-8000-000000000002", "frontageFt": 72.5}'

curl -s http://localhost:8080/civil.public.parcels.v1.ParcelsService/GetFrontageFtByParcelId \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"parcelIds": ["a0000000-0000-4000-8000-000000000002"]}'
```

Fetch a vector tile that covers the synthetic parcels:

```sh
curl -s -o tile.mvt -w '%{http_code} %{content_type} %{size_download} bytes\n' \
  -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/tiles/get_parcel_tiles/14/1820/10181
```

Instance metadata needs no token:

```sh
curl -s http://localhost:8080/civil.public.instance.v1.InstanceService/GetInstanceMetadata \
  -H 'Content-Type: application/json' -d '{}'
```

## Development

Tool versions (Go, buf, Node) are pinned in `mise.toml`. Install
[mise](https://mise.jdx.dev), then:

```sh
mise trust && mise install
mise run generate          # regenerate code from api/proto
mise run check             # build, vet, buf lint, unit and handler tests
mise run test:integration  # reader, writer and migration tests (needs Docker)
mise run test:e2e          # smoke tests against a running compose stack
```

Generated Go code under `api/gen/` and `internal/gen/` is committed. Edit the
protos and run `mise run generate`; don't edit generated files.

Commit messages and PR titles follow
[Conventional Commits](https://www.conventionalcommits.org). Releases come from
them.

## Releases

[release-please](https://github.com/googleapis/release-please) cuts two
independent release streams from `main`:

| Stream | Tag | Publishes |
|---|---|---|
| App | `vX.Y.Z` | `ghcr.io/civil-labs/civil-os/{gateway,database-reader,database-writer,tile-server}:vX.Y.Z`, plus a database-migrations ConfigMap on the GitHub Release |
| Public API | `public-api-vX.Y.Z` | `@civil-labs/civil-os-api` on npm, plus `openapi.yaml` on the GitHub Release |

Every merge to `main` also publishes `:sha-<short>` images. There is no
`latest` tag.

## License

[MIT](LICENSE)
