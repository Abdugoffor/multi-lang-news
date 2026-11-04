# Project Instructions

## General rules

- Existing project architecture and coding style must be preserved.
- Before editing, inspect neighboring files and follow their existing style.
- Make the smallest change necessary for the requested task.
- Do not refactor, rename, move, or reformat unrelated code.
- Do not introduce a new architecture, package, dependency, or abstraction unless explicitly requested.
- Preserve all user-written changes.
- Ask before making an architecture-level change when the requirement is unclear.

## Language

- Communicate with the user in Uzbek.
- Keep code, identifiers, database names, routes, and JSON fields in English.

## Architecture

Each module follows this structure:

```text
module/<module_name>/
├── cmd.go
├── dto/
│   ├── req_dto.go
│   └── res_dto.go
├── handler/
│   └── <module_name>_handler.go
└── service/
    └── <module_name>_service.go
```

Responsibilities:

- `cmd.go`: register module routes.
- `dto`: contain request, response, and filter structs.
- `handler`: parse and validate HTTP input, call the service, and write HTTP responses.
- `service`: contain business logic and PostgreSQL queries.
- `helper`: contain reusable project-level helpers.
- Never put SQL queries in handlers.
- Never put HTTP-specific logic in services.

## Go formatting style

Preserve the existing formatting style exactly.

This project intentionally uses standalone scoped blocks around error checks:

```go
res, err := handler.service.Show(r.Context(), id)
{
	if err != nil {
		writeServiceError(w, err)
		return
	}
}
```

Use the same style when adding similar code:

```go
value, err := strconv.ParseInt(rawValue, 10, 64)
{
	if err != nil || value < 1 {
		helper.JSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid value",
		})
		return
	}
}
```

- Keep blank lines consistent with the surrounding code.
- Keep braces in the same style as neighboring functions.
- Do not remove standalone `{ ... }` blocks merely because they are optional in Go.
- Do not rewrite existing functions solely to apply conventional Go style.
- Do not combine existing multiline statements into one line.
- Run `gofmt` after editing Go files.
- Review changes after `gofmt` and ensure unrelated formatting was not changed.

## Handler rules

Handlers must:

1. Read route or query parameters.
2. Validate parameters.
3. Decode and validate request DTOs.
4. Call a service method.
5. Return the appropriate HTTP response.

Example error response:

```go
helper.JSON(w, http.StatusBadRequest, map[string]string{
	"error": "invalid cursor",
})
```

Status codes:

- Use `400` for invalid input.
- Use `404` when a record does not exist.
- Use `201` after successful creation.
- Use `204` after successful deletion.
- Use `500` for unexpected service or database errors.

## Service rules

- Use `context.Context` for database operations.
- Use `pgxpool.Pool` for PostgreSQL access.
- Always use SQL parameters such as `$1` and `$2`; never concatenate user input into SQL.
- Call `defer rows.Close()` after every successful query returning rows.
- Check `rows.Err()` after iterating over query results.
- Return `pgx.ErrNoRows` when a requested record does not exist.
- Keep table and column names identical to their migration definitions.
- Never change a database table name without a migration.

## Filtering

Category lists support these filters:

- `id`: exact match.
- `name`: partial, case-insensitive search using `ILIKE`.
- `slug`: partial, case-insensitive search using `ILIKE`.
- `description`: partial, case-insensitive search using `ILIKE`.
- `is_active`: exact boolean match.

Keep `is_active` as `*bool` so these states remain distinguishable:

- `nil`: do not filter by status.
- `true`: return active records only.
- `false`: return inactive records only.

## Pagination

Use cursor/keyset pagination only.

Do not use:

- `OFFSET`.
- Page-number pagination.
- `page` or `page_size` parameters.
- `COUNT(*)` solely for pagination.

The API supports adjacent navigation:

```text
direction=next
direction=prev
cursor=<record_id>
limit=<1..100>
```

Rules:

- Default direction is `next`.
- Default limit is `20`.
- Maximum limit is `100`.
- The `prev` direction requires a cursor.
- Return results in `id DESC` order.
- Use `id < cursor` for the next page.
- Use `id > cursor` for the previous page.
- Return `next_cursor`, `prev_cursor`, `has_next`, and `has_prev`.
- Fetch `limit + 1` records to determine whether another page exists.
- Do not add page-number navigation unless explicitly requested.

## Performance

- Assume database tables may contain millions of records.
- Prefer indexed cursor queries over `OFFSET`.
- Preserve search and pagination indexes.
- Avoid unnecessary `COUNT(*)` queries.
- Never load all records into memory.
- Limit list responses to at most 100 records.
- Use PostgreSQL trigram indexes for `%text%` searches.

## Scope protection

Before changing code:

1. Read the target file.
2. Read its related DTO, handler, service, route registration, and migration.
3. Identify the existing naming and formatting style.
4. Change only files required by the request.

Never silently:

- Rename DTOs, routes, tables, columns, packages, or methods.
- Replace cursor pagination with offset pagination.
- Add a second pagination implementation.
- Modify CRUD behavior unrelated to the request.
- Remove user-written formatting blocks.
- Add authentication middleware unless requested.
- Change API response fields unless requested.

## Verification

After every Go code change:

1. Run `gofmt` only on changed Go files.
2. Run `go test ./...`.
3. Run `go vet ./...`.

Before completing:

- Confirm the project compiles.
- Confirm only requested files changed.
- Report the changed files.
- Report which checks passed.
- Mention any check that could not be run.
