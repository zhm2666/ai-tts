package errors

import "fmt"

type Status struct {
	Code    int32  `json:"code,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

type Error struct {
	Status
	cause error
}

// 实现 error 接口
func (e *Error) Error() string {
	return fmt.Sprintf("error: code = %d reason = %s message = %s cause = %v", e.Code, e.Reason, e.Message, e.cause)
}

// 包含原始错误
func (e *Error) WithCause(cause error) *Error {
	err := clone(e)
	err.cause = cause
	return err
}

// 克隆错误对象
func clone(err *Error) *Error {
	if err == nil {
		return nil
	}

	return &Error{
		cause: err.cause,
		Status: Status{
			Code:    err.Code,
			Reason:  err.Reason,
			Message: err.Message,
		},
	}
}

// 创建一个新的错误对象
func New(code int, reason, message string) *Error {
	return &Error{
		Status: Status{
			Code:    int32(code),
			Message: message,
			Reason:  reason,
		},
	}
}

// 获取错误的原因
func Reason(err error) string {
	if err == nil {
		return ""
	}
	if e, ok := err.(*Error); ok {
		return e.Reason
	}
	return ""
}

// 获取错误码
func Code(err error) int {
	if err == nil {
		return CodeOK
	}
	if e, ok := err.(*Error); ok {
		return int(e.Code)
	}
	return CodeInternal
}

// 判断两个错误是否相同
func (e *Error) Is(err error) bool {
	if err == nil || e == nil {
		return false
	}
	cusErr, ok := err.(*Error)
	if !ok {
		return false
	}
	return e.Code == cusErr.Code && e.Reason == cusErr.Reason
}
