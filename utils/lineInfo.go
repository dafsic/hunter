package utils

import (
	"runtime"
	"strconv"
	"strings"
)

// LineInfo 返回调用此函数的代码所在函数、文件、行号
func LineInfo() string {
	function := "xxx"
	pc, file, line, ok := runtime.Caller(1)
	if !ok {
		file = "???"
		line = 0
	}
	function = runtime.FuncForPC(pc).Name()

	return strings.Join(ConcatStrings(" -> ", function, file, strconv.Itoa(line)), ":")
}
