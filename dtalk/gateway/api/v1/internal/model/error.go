package model

import "errors"

var (
	ErrPermission      = errors.New("权限不足")
	ErrSessionMismatch = errors.New("不属于当前会话")
)
