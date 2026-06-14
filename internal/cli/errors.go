package cli

import (
	"context"
	"errors"

	"zlan/internal/transport"
)

// 退出码(SPEC §13)。
const (
	ExitOK        = 0
	ExitGeneral   = 1
	ExitUsage     = 2
	ExitNotFound  = 3
	ExitTimeout   = 4
	ExitProtocol  = 5
	ExitDenied    = 6
	ExitPartial   = 7
	ExitCancelled = 130
)

// ExitError 携带显式退出码;其余错误由 ExitCode 归类。
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string { return e.Err.Error() }
func (e *ExitError) Unwrap() error { return e.Err }

func exitErr(code int, err error) *ExitError { return &ExitError{Code: code, Err: err} }

// ExitCode 把错误映射为进程退出码。
func ExitCode(err error) int {
	if ee, ok := errors.AsType[*ExitError](err); ok {
		return ee.Code
	}
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return ExitCancelled
	case errors.Is(err, transport.ErrNoDevice):
		return ExitNotFound
	case errors.Is(err, transport.ErrPermission):
		return ExitDenied
	}
	return ExitGeneral
}
