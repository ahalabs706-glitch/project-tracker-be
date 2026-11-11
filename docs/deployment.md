# Deployment

## Local development

```powershell
docker compose up --build
```

The local stack includes the API, migration runner, PostgreSQL, Redis, and
notification worker. `APP_PORT` changes the host port; the API listens on port
4000 inside the Compose network. Demo records are seeded only outside
production, and can be disabled with `SEED_DEMO_DATA=false`.

## Production Compose

Set production secrets and connection settings in the deployment environment;
do not commit them to `.env` or source control. The production Compose file
requires `DATABASE_URL`, `POSTGRES_PASSWORD`, `JWT_ACCESS_SECRET`,
`REDIS_PASSWORD`, and `WEB_ORIGIN`. `DATABASE_URL` must refer to the Compose
PostgreSQL service or configured production database and match the configured
database credentials. URL-encode reserved characters in its username/password.

`JWT_ACCESS_SECRET` and `REDIS_PASSWORD` must be unique strong secrets. SMTP
settings are optional; when enabling email delivery, provide the SMTP host,
sender, and optional credentials together. Demo seeding is disabled.

Validate and launch with:

```sh
docker compose -f docker-compose.prod.yml config
docker compose -f docker-compose.prod.yml up -d --build
```

PostgreSQL and Redis are not published to host ports in the production stack.
Persistent data uses named volumes. Configure backups and TLS termination in
the hosting environment before exposing the API publicly. The API container
runs as a non-root user, has a read-only root filesystem, and handles
`SIGTERM` for graceful shutdown.

Run database migrations through the `db-setup` service. Review migration
rollback and backup procedures before applying schema changes to production.
