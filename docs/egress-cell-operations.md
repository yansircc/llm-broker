# Egress Cell Operations

## Invariant

- stable axis: `accounts.cell_id` is the account-to-egress-cell binding.
- change axis: cells can be created, updated, unbound from accounts, then deleted.
- invariant: a cell can be deleted only when no account is bound to its canonical ID.

Canonical `cell_id` means leading and trailing whitespace is not semantic. Old dirty account rows such as `" cell-http "` still refer to `cell-http`.

## Delete Cell

Use the admin API:

```bash
curl -X DELETE "$BASE_URL/admin/egress/cells/$CELL_ID" \
  -H "Authorization: Bearer $API_TOKEN"
```

Expected responses:

- `200`: the cell existed and was deleted.
- `404`: the cell does not exist.
- `409`: at least one account is still bound to the cell.

The UI exposes the same operation on the cell detail page:

```text
/cells/{id}
```

The delete action is shown only when the current cell list projection reports zero bound accounts. The backend remains the source of truth; if an account is rebound after the page loads, the DELETE request returns `409` and the cell is kept.

## Unbind First

Before deleting a bound cell, move each account to another cell or to legacy direct:

```bash
curl -X POST "$BASE_URL/admin/accounts/$ACCOUNT_ID/cell" \
  -H "Authorization: Bearer $API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"cell_id":""}'
```

Then rerun:

```bash
curl "$BASE_URL/admin/egress/cells" \
  -H "Authorization: Bearer $API_TOKEN"
```

Only delete after the target cell has an empty `accounts` array.

## Boundary

Do not delete cells by editing SQLite directly. The active process owns the in-memory pool projection, and blue/green deployments make direct DB writes ambiguous. Use the admin API so the pool, store, dashboard projection, and cell detail page share one state transition.

Do not treat whitespace variants as separate cells. The pool canonicalizes account `cell_id` values for binding validation, delete protection, and account/cell list projection.

## Verification

For this feature, the regression boundary is:

```bash
go test ./internal/pool ./internal/server -run 'Test(DeleteCell|DeleteEgressCell|BindAccountCell|ListEgressCells|CreateExchangedAccount|UpdateExchangedAccount)' -count=1
go test ./... -count=1
go test -race ./internal/pool ./internal/server -run 'Test(BindAccountCell|Update_RejectsCellDeletedAfterPreflight|Add_RejectsCellDeletedAfterPreflight)' -count=1
go build ./...
go vet ./...
cd web && npm run build
git diff --check HEAD^..HEAD
```
