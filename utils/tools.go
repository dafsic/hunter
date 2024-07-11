package utils

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"math"
	"math/rand"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"
)

// 获取 hamc sha256 算法的签名字符串 base64编码
func GetHamcSha256Base64SignStr(message, secretKey string) (sgin string) {
	mac := hmac.New(sha256.New, []byte(secretKey))
	_, err := mac.Write([]byte(message))
	if err != nil {
		return ""
	}
	sgin = base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return
}

// 获取 ed25519 算法的签名字符串 base64编码
func GetED25519Base64SignStr(message []byte, privateKey ed25519.PrivateKey) (sgin string) {
	return base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, message))
}

// 获取 hamc sha256 算法签名的字符串 16进制编码
func GetHamcSha256HexEncodeSignStr(message, secretKey string) (sgin string) {
	mac := hmac.New(sha256.New, []byte(secretKey))
	_, err := mac.Write([]byte(message))
	if err != nil {
		return ""
	}
	sgin = hex.EncodeToString(mac.Sum(nil))
	return
}

// 生成指定长度的随机字符串
func GenerateRandomString(length int) string {
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return string(result)
}

// 将map转化为符合号'='和'&'连接的字符串
func GetAndEqJionString(payload map[string]string) (formattedString string) {

	if payload == nil {
		return
	}

	keys := make([]string, 0, len(payload))
	for k := range payload {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	sortedDict := make([]string, len(keys))
	for i, k := range keys {
		sortedDict[i] = k + "=" + payload[k]
	}
	formattedString = strings.Join(sortedDict, "&")
	return formattedString
}

// 调整有效位数（舍去）
// value: 312.12345999999998, decimalPlaces: -2 => 300
// value: 312.12345999999998, decimalPlaces: -1 => 310
// value: 312.12345999999998, decimalPlaces: 0 => 312
// value: 312.12345999999998, decimalPlaces: 1 => 312.1
// value: 312.12345999999998, decimalPlaces: 2 => 312.12
// value: 312.12345999999998, decimalPlaces: 5 => 312.12346
func FormatFloat(value float64, decimalPlaces int32) float64 {
	str := strconv.FormatFloat(value, 'f', -1, 64)
	if !strings.Contains(str, ".") {
		str += "."
	}

	if str == "NaN" || str == "Inf" || str == "-Inf" {
		return 0
	}

	if len(str) > 18 {
		str = str[:18]
	}
	padding := strings.Repeat("0", 18-len(str))
	formatted := str + padding

	// 计算小数点部分的位数
	parts := strings.Split(formatted, ".")
	floatLen := 0
	if len(parts) > 1 {
		floatLen = len(parts[1])
	}

	// value += math.Pow10(-floatLen + 1)
	value += math.Pow10(-floatLen + 3)

	str = strconv.FormatFloat(value, 'f', -1, 64)
	if !strings.Contains(str, ".") {
		str += ".0"
	}
	parts = strings.Split(str, ".")
	intStr := parts[0]
	floatStr := parts[1]
	floatStr += strings.Repeat("0", 18)

	if decimalPlaces > 0 {
		v, _ := strconv.ParseFloat(intStr+"."+floatStr[:decimalPlaces], 64)
		return v
	} else if decimalPlaces < 0 {

		if len(intStr) <= -int(decimalPlaces) {
			return 0
		}

		intStr2 := intStr[:len(intStr)+int(decimalPlaces)]
		v, _ := strconv.ParseFloat(intStr2+strings.Repeat("0", len(intStr)-len(intStr2)), 64)
		return v
	} else {
		v, _ := strconv.ParseFloat(intStr, 64)
		return v
	}
}

// 获取有效位数（将"0.001"转换为3）
// "0.001" => 3
// "0.01" => 2
// "1" => 0
// "100" => -2
func GetDecimalPlaces(str string) int32 {

	// 如果字符串包含小数点
	if strings.Contains(str, ".") {
		for strings.HasSuffix(str, "0") {
			str = str[:len(str)-1]
		}

		// 分割字符串，获取小数点后的部分
		parts := strings.Split(str, ".")
		// 返回小数点后的位数
		return int32(len(parts[1]))

	} else {
		// 如果字符串不包含小数点，计算末尾的零的数量
		zeroCount := 0
		for i := len(str) - 1; i >= 0; i-- {
			if str[i] == '0' {
				zeroCount++
			} else {
				break
			}
		}
		// 返回零的数量的负值
		return int32(-zeroCount)
	}
}

// 根据有效位数获取最小步进（将3转换为0.001）
func GetMinStep(decimalPlaces int32) float64 {
	return math.Pow(10, float64(-decimalPlaces))
}

// float64转string
func Float64ToString(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}
func StringToFloat64(num string) float64 {
	fnum, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0
	}
	return fnum
}

// 将int64毫秒级时间戳转换为"2006-01-02 15:04:05.000"GTC时间格式
func Int64ToTimeFormat(timestamp int64) string {
	sec := timestamp / 1000
	nsec := (timestamp % 1000) * int64(time.Millisecond)
	t := time.Unix(sec, nsec)
	return t.Format("2006-01-02 15:04:05.000")
}

// 将int64毫秒级时间间隔转换为时分秒的格式
func Int64ToTimeDuration(timestamp int64) string {
	d := time.Duration(timestamp) * time.Millisecond
	// 提取小时、分钟和秒
	hours := d / time.Hour
	d %= time.Hour
	minutes := d / time.Minute
	d %= time.Minute
	seconds := d / time.Second

	return strconv.Itoa(int(hours)) + "时" + strconv.Itoa(int(minutes)) + "分" + strconv.Itoa(int(seconds)) + "秒"
}

// 获取本机所有ip
func GetAllLocalIPs() ([]string, error) {
	ipArr := make([]string, 0)
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ipArr, err
	}
	for _, address := range addrs {
		// 检查ip地址判断是否回环地址
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ipArr = append(ipArr, ipnet.IP.String())
			}
		}
	}
	return ipArr, nil
}
