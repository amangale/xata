package serverless

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"syscall"

	"xata/services/gateway/serverless/spec"
	"xata/services/gateway/session"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/labstack/echo/v4"
)

var (
	ErrInvalidRequest          = errors.New("invalid request")
	ErrMissingQuery            = errors.New("missing query")
	ErrMissingConnectionString = errors.New("missing Connection-String header")
	ErrInvalidConnectionString = errors.New("invalid connection string format")
	ErrResponseTooLarge        = errors.New("response too large")
)

func errorResponse(code, message string) *spec.ErrorResponse {
	return &spec.ErrorResponse{Message: message, Code: &code}
}

func pgWireError(pgErr *pgconn.PgError) *pgproto3.ErrorResponse {
	return &pgproto3.ErrorResponse{
		Severity:         pgErr.Severity,
		Code:             pgErr.Code,
		Message:          pgErr.Message,
		Detail:           pgErr.Detail,
		Hint:             pgErr.Hint,
		Position:         pgErr.Position,
		InternalPosition: pgErr.InternalPosition,
		InternalQuery:    pgErr.InternalQuery,
		Where:            pgErr.Where,
		SchemaName:       pgErr.SchemaName,
		TableName:        pgErr.TableName,
		ColumnName:       pgErr.ColumnName,
		DataTypeName:     pgErr.DataTypeName,
		ConstraintName:   pgErr.ConstraintName,
		File:             pgErr.File,
		Line:             pgErr.Line,
		Routine:          pgErr.Routine,
	}
}

func pgErrorResponse(pgErr *pgconn.PgError) *spec.ErrorResponse {
	resp := &spec.ErrorResponse{
		Message: pgErr.Message,
	}
	if pgErr.Code != "" {
		resp.Code = &pgErr.Code
	}
	if pgErr.Severity != "" {
		resp.Severity = &pgErr.Severity
	}
	if pgErr.Detail != "" {
		resp.Detail = &pgErr.Detail
	}
	if pgErr.Hint != "" {
		resp.Hint = &pgErr.Hint
	}
	if pgErr.InternalQuery != "" {
		resp.InternalQuery = &pgErr.InternalQuery
	}
	if pgErr.Where != "" {
		resp.Where = &pgErr.Where
	}
	if pgErr.SchemaName != "" {
		resp.Schema = &pgErr.SchemaName
	}
	if pgErr.TableName != "" {
		resp.Table = &pgErr.TableName
	}
	if pgErr.ColumnName != "" {
		resp.Column = &pgErr.ColumnName
	}
	if pgErr.DataTypeName != "" {
		resp.DataType = &pgErr.DataTypeName
	}
	if pgErr.ConstraintName != "" {
		resp.Constraint = &pgErr.ConstraintName
	}
	if pgErr.File != "" {
		resp.File = &pgErr.File
	}
	if pgErr.Routine != "" {
		resp.Routine = &pgErr.Routine
	}
	if pgErr.Position > 0 {
		resp.Position = new(fmt.Sprintf("%d", pgErr.Position))
	}
	if pgErr.InternalPosition > 0 {
		resp.InternalPosition = new(fmt.Sprintf("%d", pgErr.InternalPosition))
	}
	if pgErr.Line > 0 {
		resp.Line = new(fmt.Sprintf("%d", pgErr.Line))
	}
	return resp
}

// handlePgError maps the error classification to an HTTP response.
func handlePgError(c echo.Context, err error) error {
	switch classifyError(err) {
	case "response_too_large":
		return c.JSON(http.StatusInsufficientStorage, errorResponse("RESPONSE_TOO_LARGE", err.Error()))
	case "timeout":
		return c.JSON(http.StatusGatewayTimeout, errorResponse("QUERY_TIMEOUT", "query exceeded the timeout limit"))
	case "canceled":
		return nil
	case "hibernated":
		return c.JSON(http.StatusConflict, errorResponse("BRANCH_HIBERNATED", "branch is hibernated, reactivate it to continue"))
	case "branch_not_found":
		return c.JSON(http.StatusNotFound, errorResponse("BRANCH_NOT_FOUND", "branch not found"))
	case "pg_error":
		var pgErr *pgconn.PgError
		errors.As(err, &pgErr)
		return c.JSON(http.StatusBadRequest, pgErrorResponse(pgErr))
	case "client":
		return c.JSON(http.StatusBadRequest, errorResponse("08P01", err.Error()))
	default:
		return c.JSON(http.StatusInternalServerError, errorResponse("XX000", sanitizeError(err)))
	}
}

// isPgxClientError detects parameter binding errors raised by pgx before the
// query reaches PostgreSQL. These are definitively client errors (wrong number
// of parameters, etc.) that should return 400.
func isPgxClientError(err error) bool {
	msg := err.Error()
	return strings.HasPrefix(msg, "expected ") ||
		strings.HasPrefix(msg, "unused argument")
}

// classifyError returns a short category string for an error, suitable for
// use as a metric attribute or structured log field.
func classifyError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, session.ErrBranchHibernated) {
		return "hibernated"
	}
	if errors.Is(err, session.ErrBranchNotFound) {
		return "branch_not_found"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	if errors.Is(err, ErrResponseTooLarge) {
		return "response_too_large"
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return "pg_error"
	}
	if isPgxClientError(err) {
		return "client"
	}
	if isConnectionError(err) {
		return "connection"
	}
	return "other"
}

func isConnectionError(err error) bool {
	if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ECONNRESET) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	return strings.Contains(err.Error(), "connect:")
}

// sanitizeError strips credentials from error messages to prevent leaking
// passwords in logs or HTTP responses. Connection errors from pgx can contain
// the full connection URI including user:password.
func sanitizeError(err error) string {
	msg := err.Error()
	i := strings.Index(msg, "://")
	if i == -1 {
		return msg
	}
	at := strings.Index(msg[i:], "@")
	if at == -1 {
		return msg
	}
	return msg[:i+3] + "REDACTED" + msg[i+at:]
}
