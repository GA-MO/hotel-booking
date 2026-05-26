# ADR-0003 — Self-host on Hetzner Cloud

- **Status:** Accepted
- **Date:** 2026-05-26
- **Deciders:** team
- **Related:** plan.md §2.1 (Hosting), §2.2 (Self-host Considerations), §3 (Architecture)

## Context

We need to host an API, a Next.js frontend, Postgres, Redis, an image
proxy, object storage, and a worker fleet. We have a 2–3 person team and a
target operating cost during Phase 0–1 of well under €100/month. The
business model (subscription with a free year — ADR-0001) means we are
burning, not earning, for the first 12 months and every euro of fixed
infra cost extends or shortens our runway.

The shopping list breaks down roughly as:

- **All managed (Vercel + Fly.io + Neon + Upstash + Cloudinary):** ~$50–150/mo at our
  scale, ~zero ops time, vendor-lock per service, costs spike as we grow.
- **All AWS (ECS + RDS + ElastiCache + S3 + CloudFront):** "enterprise"
  defaults, heavy IAM/VPC ops, surprise bills, overkill for our footprint.
- **Self-host on a budget VPS (Hetzner / Hetzner+Cloudflare):** ~€30–40/mo
  for two CPX21 nodes, full control, but we own ops.
- **Singapore-region VPS (DigitalOcean / Vultr):** 2–3× Hetzner price for
  ~30 ms latency to Thai users vs ~250 ms from Helsinki.

Our target market is Thai small hotels with guests booking from TH and abroad.
The dominant page is the landing page, which is heavily cacheable via CDN, so
origin latency matters far less than it would for a chatty SPA.

## Decision

Self-host the entire stack on Hetzner Cloud in Helsinki: 2× CPX21 (4 vCPU,
8 GB) in Phase 1, all services as `docker compose` units behind Caddy +
Cloudflare. Daily Postgres backups to Backblaze B2. Scale vertically first,
then horizontally, before considering managed services.

## Consequences

### Positive
- Total infra ≈ €30–40/mo Phase 1 — vs $50–150/mo for an equivalent managed stack.
- One environment, one runbook, no per-vendor surprises.
- Cloudflare in front absorbs DDoS / WAF / TLS / static caching for free.
- Egress is cheap to absurd (Hetzner includes 20 TB/mo on CPX21).

### Negative / trade-offs
- Plan.md §2.2 explicitly budgets **20–30 % of dev time in Phase 0–1 on ops** — backup verification, secret rotation, kernel upgrades, alert tuning.
- Helsinki origin → ~250 ms latency to Thailand. Mitigated by Cloudflare edge + Next.js ISR + aggressive cache headers; the booking POST hop is the only un-cacheable user-facing call.
- We are responsible for security updates, disk monitoring, and restore tests (target: at least monthly per plan.md §2.2).
- No automatic failover. A single-AZ outage takes us down until restored.

### Neutral
- Stack is portable — every service is a normal Docker container. If/when we outgrow Hetzner we can lift-and-shift to k8s/k3s, AWS, or managed services without rewriting application code.

## Alternatives considered

### Alt 1: Fully managed (Vercel + Fly + Neon + Upstash + Cloudinary)
Rejected. Cheapest viable bundle is ~$50–150/mo before any traffic — the marginal cost on a Hetzner box is closer to zero. Vendor sprawl: five dashboards, five billing pages, five outage Twitter accounts to follow.

### Alt 2: All-AWS (ECS + RDS + ElastiCache + S3 + CloudFront)
Rejected. Overkill for a 2–3 dev team. IAM, VPC, and the bill-shock surface area cost more dev time than self-host ops would.

### Alt 3: Singapore VPS (DigitalOcean / Vultr SGP)
Rejected for Phase 1. 2–3× the cost for ~30 ms latency to TH — most of which the Cloudflare edge erases for read traffic anyway. Reconsider if booking POST latency becomes a measurable conversion drag.

## Notes

- Detailed deployment is in `infra/README.md` and `docker-compose.prod.yml`.
- Re-evaluate when MRR > ~€500/mo or when ops time exceeds 30 % of any sprint.
