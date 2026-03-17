package errors

const (
	// 2xx - 成功
	CodeOK = 200

	// 4xx - 客户端错误
	CodeBadRequest       = 400 // 请求参数错误
	CodeUnauthorized     = 401 // 未认证
	CodeForbidden        = 403 // 无权限
	CodeNotFound         = 404 // 资源不存在
	CodeMethodNotAllowed = 405 // 方法不允许
	CodeConflict         = 409 // 资源冲突
	CodeTooManyRequests  = 429 // 请求过于频繁

	// 5xx - 服务器错误
	CodeInternal       = 500 // 服务器内部错误
	CodeNotImplemented = 501 // 功能未实现
	CodeBadGateway     = 502 // 网关错误
	CodeUnavailable    = 503 // 服务不可用
	CodeTimeout        = 504 // 请求超时
)
