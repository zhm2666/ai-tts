package errors

// 包装错误为自定义错误类型
func WrapError(err error) *Error {
	if err == nil {
		return nil
	}
	cusErr, ok := err.(*Error)
	if ok {
		return cusErr
	}
	return New(CodeInternal, "COMMON_INTERNAL_ERROR", "服务器内部错误").WithCause(err)
}

// 包装错误消息为自定义错误类型
func WrapMsg(msg string) *Error {
	return New(CodeInternal, "COMMON_INTERNAL_ERROR", msg)
}

// IsNotFound 判断是否为"不存在"错误
func IsNotFound(err error) bool {
	return Code(err) == CodeNotFound
}

// IsConflict 判断是否为"冲突"错误, 如数据已存在等
func IsConflict(err error) bool {
	return Code(err) == CodeConflict
}

// IsUnauthorized 判断是否为"未授权"错误
func IsUnauthorized(err error) bool {
	return Code(err) == CodeUnauthorized
}
