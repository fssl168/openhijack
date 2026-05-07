package errors

import (
	"fmt"
)

// ErrorCode 定义应用错误码
type ErrorCode int

const (
	// 成功 (0)
	ErrSuccess ErrorCode = iota

	// 用户输入错误 (1001-1099)
	ErrInvalidInput
	ErrRequiredFieldMissing
	ErrInvalidFormatValue
	ErrValueOutOfRange

	// 资源错误 (1101-1199)
	ErrNotFound
	ErrAlreadyExistsResource
	ErrResourceLocked

	// 权限错误 (1201-1299)
	ErrPermissionDenied
	ErrAuthenticationFailed
	ErrUnauthorized
	ErrForbidden

	// 网络错误 (1301-1399)
	ErrNetworkConnectionFailed
	ErrTimeoutError
	ErrTLSHandshakeError

	// 配置错误 (1401-1499)
	ErrConfigFileNotFound
	ErrConfigParseFailed
	ErrConfigValidationFailed

	// 服务错误 (1501-1599)
	ErrServiceNotRunning
	ErrServiceStartFailed
	ErrServiceStopFailed
	ErrPortBindFailed

	// 内部错误 (1601-1699)
	ErrInternalError
	ErrNotImplemented
	ErrDatabaseError
	ErrFileOperationError
)

// AppError 统一的应用错误结构
type AppError struct {
	Code      ErrorCode `json:"code"`
	Message   string     `json:"message"`            // 技术性消息（用于日志）
	UserMsg   string     `json:"user_message,omitempty"` // 用户友好消息（用于显示）
	Details   string     `json:"details,omitempty"`       // 详细信息
	Cause     error      `json:"-"`                    // 原始错误
	Field     string     `json:"field,omitempty"`        // 关联的字段名（用于表单验证）
}

// Error 实现 error 接口
func (e *AppError) Error() string {
	if e.UserMsg != "" {
		return e.UserMsg
	}
	return e.Message
}

// Unwrap 支持错误链解包
func (e *AppError) Unwrap() error {
	return e.Cause
}

// Is 实现 errors.Is 接口，用于标准库错误比较
func (e *AppError) Is(target error) bool {
	if target == nil {
		return false
	}

	if appErr, ok := target.(*AppError); ok {
		return e.Code == appErr.Code
	}

	return false
}

// IsCode 检查是否为特定错误码（便捷方法）
func (e *AppError) IsCode(code ErrorCode) bool {
	return e.Code == code
}

// New 创建新的应用错误
func New(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// WithUserMsg 设置用户友好消息
func (e *AppError) WithUserMsg(userMsg string) *AppError {
	e.UserMsg = userMsg
	return e
}

// WithDetails 设置详细信息
func (e *AppError) WithDetails(details string) *AppError {
	e.Details = details
	return e
}

// WithCause 设置原始错误
func (e *AppError) WithCause(cause error) *AppError {
	e.Cause = cause
	return e
}

// WithField 设置关联字段
func (e *AppError) WithField(field string) *AppError {
	e.Field = field
	return e
}

// 预定义的错误构造函数

// 输入验证错误
func ErrInvalidInputf(format string, args ...interface{}) *AppError {
	return &AppError{
		Code:    ErrInvalidInput,
		Message: fmt.Sprintf(format, args...),
		UserMsg: "输入数据无效，请检查后重试",
	}
}

func ErrRequiredField(field string) *AppError {
	return &AppError{
		Code:    ErrRequiredFieldMissing,
		Message: fmt.Sprintf("required field '%s' is missing", field),
		UserMsg: fmt.Sprintf("'%s' 为必填项", field),
		Field:  field,
	}
}

func ErrInvalidFieldValue(field, format string) *AppError {
	return &AppError{
		Code:    ErrInvalidFormatValue,
		Message: fmt.Sprintf("field '%s' has invalid format, expected %s", field, format),
		UserMsg: fmt.Sprintf("%s 格式不正确，应为 %s", field, format),
		Field:  field,
	}
}

// 资源错误
func ErrNotFoundf(resource string, args ...interface{}) *AppError {
	return &AppError{
		Code:    ErrNotFound,
		Message: fmt.Sprintf(resource, args...),
		UserMsg: "请求的资源不存在",
	}
}

func ErrAlreadyExists(resource string) *AppError {
	return &AppError{
		Code:    ErrAlreadyExistsResource,
		Message: fmt.Sprintf("resource already exists: %s", resource),
		UserMsg: fmt.Sprintf("%s 已存在", resource),
	}
}

// 权限错误
func ErrPermissionDeniedf(format string, args ...interface{}) *AppError {
	return &AppError{
		Code:    ErrPermissionDenied,
		Message: fmt.Sprintf(format, args...),
		UserMsg: "权限不足，请检查文件权限或使用 sudo 运行",
	}
}

func NewErrAuthenticationFailed(msg string) *AppError {
	return &AppError{
		Code:    ErrAuthenticationFailed,
		Message: msg,
		UserMsg: "认证失败，请检查密钥是否正确",
	}
}

// 网络错误
func NewErrNetworkConnectionFailed(err error) *AppError {
	return &AppError{
		Code:    ErrNetworkConnectionFailed,
		Message: fmt.Sprintf("connection failed: %v", err),
		UserMsg: "网络连接失败，请检查网络设置",
		Cause:   err,
	}
}

func ErrTimeout(operation string) *AppError {
	return &AppError{
		Code:    ErrTimeoutError,
		Message: fmt.Sprintf("operation timed out: %s", operation),
		UserMsg: fmt.Sprintf("%s 超时，请稍后重试", operation),
	}
}

// 配置错误
func NewErrConfigFileNotFound(path string) *AppError {
	return &AppError{
		Code:    ErrConfigFileNotFound,
		Message: fmt.Sprintf("config file not found: %s", path),
		UserMsg: fmt.Sprintf("配置文件不存在: %s", path),
	}
}

func ErrConfigParseError(path string, err error) *AppError {
	return &AppError{
		Code:    ErrConfigParseFailed,
		Message: fmt.Sprintf("failed to parse config %s: %v", path, err),
		UserMsg: "配置文件格式错误，请检查语法",
		Cause:   err,
	}
}

// 服务错误
func NewErrServiceStartFailed(port int, err error) *AppError {
	return &AppError{
		Code:    ErrServiceStartFailed,
		Message: fmt.Sprintf("failed to start service on port %d: %v", port, err),
		UserMsg: fmt.Sprintf("服务启动失败 (端口 %d)", port),
		Cause:   err,
	}
}

func NewErrPortBindFailed(port int, err error) *AppError {
	msg := fmt.Sprintf("failed to bind port %d: %v", port, err)
	userMsg := fmt.Sprintf("端口 %d 绑定失败", port)
	
	if port < 1024 {
		userMsg += "，建议使用 ≥1024 的端口或以 sudo 运行"
	}
	
	return &AppError{
		Code:    ErrPortBindFailed,
		Message: msg,
		UserMsg: userMsg,
		Cause:   err,
	}
}

// 内部错误
func ErrInternalf(format string, args ...interface{}) *AppError {
	return &AppError{
		Code:    ErrInternalError,
		Message: fmt.Sprintf(format, args...),
		UserMsg: "内部错误，请联系开发者",
	}
}

// Wrap 包装已有错误
func Wrap(cause error, code ErrorCode, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// AsAppError 尝试将 error 转换为 *AppError
func AsAppError(err error) (*AppError, bool) {
	if appErr, ok := err.(*AppError); ok {
		return appErr, true
	}
	return nil, false
}

// IsErrorCode 检查错误是否包含特定错误码
func IsErrorCode(err error, code ErrorCode) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code == code
	}
	return false
}

// GetUserMessage 获取用户友好的错误消息
func GetUserMessage(err error) string {
	if appErr, ok := err.(*AppError); ok {
		if appErr.UserMsg != "" {
			return appErr.UserMsg
		}
		return appErr.Message
	}
	return err.Error()
}

// ToUserString 将错误转换为适合前端显示的字符串（向后兼容旧的 string 返回值）
func ToUserString(err error) string {
	if err == nil {
		return ""
	}
	return GetUserMessage(err)
}
