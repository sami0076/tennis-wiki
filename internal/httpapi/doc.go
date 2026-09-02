// Package httpapi holds the HTTP handlers and middleware.
//
// Cross-cutting concerns are middleware rather than per-handler code:
// request IDs propagated into slog, RFC 7807 problem+json errors, ETag
// revalidation, IP rate limiting, and CORS.
package httpapi
