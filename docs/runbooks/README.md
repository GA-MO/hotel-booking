# Runbooks

Operational playbooks. Each runbook is a step-by-step procedure for a specific scenario — written so an on-call (or AI assistant) can execute it without having to derive the steps from first principles.

## Index

| # | Scenario | When to use |
|---|---|---|
| [01](01-deployment.md) | First-time deployment | New server, never deployed before |
| [02](02-rollback.md) | Rollback a bad deploy | Production is broken and needs to revert |
| [03](03-restore-from-backup.md) | Restore Postgres from Backblaze B2 | Data loss / corruption / verify backup health |
| [04](04-incident-overbooking.md) | Resolve a double-booking report | Guest claims room is already occupied |
| [05](05-rotate-jwt-secret.md) | Rotate `JWT_SECRET` | Suspected token leak or scheduled rotation |

## Conventions for new runbooks

- Filename: `NN-kebab-case.md` (zero-padded sequence).
- Each runbook starts with **When to use this**, **Time estimate**, **Risk level (low / medium / high)**.
- Use numbered steps. Each step has a single concrete action with a verifiable success condition.
- Commands are copy-pasteable.
- Include **"If this step fails"** branches for the failure modes you've seen.
- End with **Postmortem template** if the incident class is recurring.

See [`infra/README.md`](../../infra/README.md) for the full deployment runbook (deployment topology + first-time server setup).
