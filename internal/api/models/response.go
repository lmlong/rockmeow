// Package models API 请求/响应模型
package models

// ErrorResponse 统一错误响应
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail 错误详情
type ErrorDetail struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// SuccessResponse 成功响应
type SuccessResponse struct {
	Message string `json:"message"`
	ID      string `json:"id,omitempty"`
}
