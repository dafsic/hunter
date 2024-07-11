package utils

import (
	"regexp"
	"strconv"
	"strings"
	"unsafe"
)

// ConcatStrings 将多个字符串合成一个字符串数组
func ConcatStrings(elems ...string) []string {
	return elems
}

// StrSplit 使用逗号或者分号对字符串进行切割
func StrSplit(s string, p ...rune) []string {
	return strings.FieldsFunc(s, func(c rune) bool { return c == ',' || c == ';' })
}

// StringToBytes 直接将字符串转换成[]byte，无内存copy
func StringToBytes(s string) []byte {
	stringHeader := unsafe.StringData(s)
	return unsafe.Slice(stringHeader, len(s))
}

// BytesToString 直接将[]byte转成字符串，无内存copy
func BytesToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// CompressStr 压缩字符串，去掉所有空白符
func CompressStr(str string) string {
	if str == "" {
		return ""
	}
	//匹配一个或多个空白符的正则表达式
	reg := regexp.MustCompile("\\s+")
	return reg.ReplaceAllString(str, "")
}

// StringToInt32 将字符串转换为int32
func StringToInt32(numStr string) int32 {
	num, _ := strconv.Atoi(numStr)
	return int32(num)
}
