package utils

import (
	"github.com/google/uuid"
)

// 获取标准的uuid字符串
func GetUUID() string {
	return uuid.New().String()
}
