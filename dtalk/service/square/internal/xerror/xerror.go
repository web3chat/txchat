package xerror

import (
	"errors"
	"fmt"
)

const (
	OK                = 0
	Internal          = -1000
	ParamInvalid      = -1012
	SignatureInvalid  = -1010
	SignatureExpired  = -1011
	NotFound          = -20404
	Forbidden         = -20403
	ContentIllegal    = -20401
	PendingReview     = -20402
	RateLimited       = -20429
	OfficialBanned    = -20405
	NotOfficial       = -20406
	CommentsOff       = -20407
	AlreadyPending    = -20408
	UserBanned        = -20409
	AlreadyReported   = -20410
	FollowSelf        = -20411
	PinLimit          = -20412
	SquareClosed      = -20413
	SquareLimited     = -20414
	EditWindowClosed  = -20415
	DuplicatePost     = -20416
	OfficialNameTaken = -20417
	ReplyUnavailable  = -20418

	AdminLocked       = -30401
	AdminBadCred      = -30402
	AdminUnauthorized = -30403
	AdminCaptcha      = -30404
)

type AppError struct {
	Code    int
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error { return e.Err }

func New(code int, msg string) *AppError {
	return &AppError{Code: code, Message: msg}
}

func Wrap(code int, msg string, err error) *AppError {
	return &AppError{Code: code, Message: msg, Err: err}
}

func AsApp(err error) (*AppError, bool) {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}

func CodeOf(err error) int {
	if ae, ok := AsApp(err); ok {
		return ae.Code
	}
	return Internal
}

func MessageOf(err error) string {
	if ae, ok := AsApp(err); ok {
		return ae.Message
	}
	if err == nil {
		return "操作成功"
	}
	return "内部错误"
}
