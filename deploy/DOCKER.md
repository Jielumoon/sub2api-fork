# Sub2API Docker Image

Sub2API is an AI API Gateway Platform for distributing and managing AI product subscription API quotas.

## Quick Start

```bash
git clone https://github.com/Jielumoon/sub2api-fork.git
cd sub2api-fork/deploy
cp .env.example .env
# Replace the placeholder POSTGRES_PASSWORD in .env; also set stable
# JWT_SECRET and TOTP_ENCRYPTION_KEY values before starting.
nano .env
docker compose up -d
docker compose logs -f sub2api
```

## Docker Compose

The included [`docker-compose.yml`](docker-compose.yml) starts Sub2API,
PostgreSQL, and Redis with named volumes. For local data directories, use
[`docker-compose.local.yml`](docker-compose.local.yml). Both pull
`ghcr.io/jielumoon/sub2api-fork:latest`.

## Startup and Database Recovery

Sub2API runs database migrations while starting. PostgreSQL may still be
recovering briefly after a host or Docker daemon restart. The application
retries transient PostgreSQL startup and connection errors with bounded
exponential backoff, then continues startup when the database is ready.
Permanent errors such as invalid credentials, migration checksum mismatches,
SQL errors, and incompatible data fail immediately.

The Compose deployment also checks PostgreSQL readiness with both `pg_isready`
and a simple SQL query. `depends_on: condition: service_healthy` helps order a
fresh Compose start, but application-level retries are still required when
Docker restores existing containers after a host restart.

## Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `POSTGRES_PASSWORD` | PostgreSQL password used by Compose and the application | Yes | - |
| `JWT_SECRET` | Stable signing secret across restarts | Recommended | Random if empty |
| `TOTP_ENCRYPTION_KEY` | Stable key for 2FA secrets | Recommended | Random if empty |
| `SERVER_PORT` | Host port mapped by Compose | No | `8080` |

## Supported Architectures

- `linux/amd64`
- `linux/arm64`

## Tags

- `latest` - Latest stable release
- `x.y.z` - Specific version
- `x.y` - Latest patch of minor version
- `x` - Latest minor of major version

## Links

- [GitHub Repository](https://github.com/Jielumoon/sub2api-fork)
- [Documentation](https://github.com/Jielumoon/sub2api-fork#readme)
