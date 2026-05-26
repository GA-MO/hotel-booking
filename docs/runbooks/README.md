# Runbooks

Step-by-step procedures for operations we've actually performed at least once. We deliberately **don't** pre-write runbooks for hypothetical incidents — speculative steps drift into wrongness and create false confidence.

When something happens for the first time, write the runbook **while** you do it.

## Current runbooks

| # | Scenario | Verified? |
|---|---|---|
| [01](01-deployment.md) | First-time deployment | ✅ (production stand-up) |
| [02](02-rollback.md) | Rollback a bad deploy | ⚠️  not yet executed in anger — verify on next deploy |

> The authoritative deployment runbook is [`infra/README.md`](../../infra/README.md). The files here are scenario-specific overlays.

## Add a runbook when…

- You just did an operation for the first time → write it down before you forget
- A near-incident exposed a gap → capture the recovery while it's fresh
- An incident occurred → write the postmortem AND the recurring fix-path

## Conventions

- Filename: `NN-kebab-case.md` (zero-padded sequence).
- Start with **When to use**, **Time estimate**, **Risk level (low / medium / high)**, and a `✅ Verified` or `⚠️ Unverified` marker.
- Use numbered steps; each step has one concrete action with a verifiable success condition.
- Include **"If this step fails"** for the failure modes you've actually hit.
- After the first execution, update the verified marker and any steps that didn't match reality.
