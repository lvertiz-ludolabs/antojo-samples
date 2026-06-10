# Organization setup use case

Code sample from **Antojo**, a restaurant-management monorepo (Go API + Nuxt front-end + PostgreSQL).

This folder documents two files from the `org` feature that implement and test the **organization onboarding** flow: creating a new tenant with an admin user and primary contact in a single atomic operation.

| File (in monorepo) | Role |
| --- | --- |
| `apps/api/internal/org/app/setup_cmd.go` | Application command — business orchestration |
| `apps/api/internal/org/app/setup_cmd_test.go` | Unit tests with mocked ports |

---

## Problem

When a restaurant signs up, the system must create several related records together:

1. **Organization** — name and public alias (e.g. `tortas-vertiz`)
2. **Admin user** — default `admin` account with hashed password
3. **Primary contact** — name, email, phone marked as main contact

If any step fails after the org is created, the database must not end up in a half-initialized state. Alias uniqueness and format must be validated before writes.

---

## Solution overview

`SetupCommand` is the **application layer** entry point. It does not handle HTTP or SQL directly.

```
HTTP handler  →  SetupCommand.Execute  →  repositories / other commands
                      ↓
                 Transactor.RunInTx (PostgreSQL transaction)
```

### What happens on `Execute`

1. **Pre-transaction checks** (outside the DB transaction):
   - Validate alias format (`utils.IsValidAlias`)
   - Ensure alias is not already taken (`FindByAlias`)

2. **Inside `RunInTx`**:
   - Create organization
   - Delegate to `CreateUserCommand` (admin user, bcrypt hashing inside that command)
   - Delegate to `CreateContactCommand` (primary contact)

3. On any error inside the transaction, everything rolls back.

4. Return a narrow `OrgDTO` (name + alias only — no internal IDs exposed unnecessarily in this response).

---

## Architecture patterns

| Pattern | How it shows up here |
| --- | --- |
| **CQRS** | `SetupCommand` is a write use case (`*_cmd.go`); reads live in separate `*_qry.go` files |
| **Hexagonal / ports** | Depends on `OrgRepositoryPort`, `Transactor`, `CreateCommandPort` interfaces — not concrete infra |
| **Feature modules** | `org` orchestrates `user` and `org_contact` via their application ports, not their repositories |
| **Transaction boundary** | `Transactor.RunInTx` + context-scoped executor — all repos in the tx share the same `*sqlx.Tx` |
| **Structured errors** | Domain failures return `shared.AppError` with HTTP code, message, and client-facing `slug` |

The HTTP handler (`setup_hdl.go`) only binds JSON, validates struct tags, and calls `Execute` — no business rules in the handler.

---

## Testing strategy (`setup_cmd_test.go`)

Tests are **fast unit tests**. They mock every port with **mockery**-generated mocks:

| Test | Scenario |
| --- | --- |
| `TestSetupOrgCommand_Execute` | Happy path — verifies orchestration order, admin role, contact fields, and DTO mapping |
| `TestSetupOrgCommand_Execute_Error` | Duplicate alias — `FindByAlias` returns existing org → error, no transaction |
| `TestSetupOrgCommand_Execute_Error_InvalidAlias` | Reserved/invalid alias (`admin`) → fails before any repository call |

The transactor mock uses `RunAndReturn` to invoke the callback synchronously, so the test exercises the full orchestration logic without a real database.

Elsewhere in the same API, **repository tests** run against a real PostgreSQL instance with golang-migrate (full `Down`/`Up` per test). This command layer stays mock-based so use-case logic can be tested in isolation.

---

## Stack context

- **Go**, Echo v4, sqlx, Squirrel, PostgreSQL
- **Validation**: go-playground/validator on HTTP inputs; additional rules in commands
- **Monorepo**: Turborepo + pnpm; TypeScript types generated from Go DTOs via **tygo**
- **E2E**: Playwright against the Nuxt restaurant app

---

## Running the tests

From the API module (requires Go 1.22+):

```bash
cd apps/api
go test ./internal/org/app/ -run TestSetupOrgCommand -v
```

No database required for these tests — only the mocked ports.

---

## Author note

This slice is representative of how every feature in the API is structured: `api/` (HTTP) → `app/` (commands/queries) → `domain/` (entities + ports) → `repository.go` (SQL). The setup flow is a good example because it crosses module boundaries while keeping dependencies pointed inward.
