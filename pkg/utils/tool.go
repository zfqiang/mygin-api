package utils

import (
	"strconv"
)

// StrToInt 获取查询参数并转换为 int，如果失败则返回默认值
func StrToInt(strVal string) (int, error) {
	val, err := strconv.Atoi(strVal)
	if err != nil {
		return 0, err
	}
	return val, nil
}

// IntToStr 转换 int 为 string
func IntToStr(val int) string {
	return strconv.Itoa(val)
}

// StrToInt64 获取查询参数并转换为 int64，如果失败则返回默认值
func StrToInt64(strVal string) (int64, error) {
	val, err := strconv.ParseInt(strVal, 10, 64)
	if err != nil {
		return 0, err
	}
	return val, nil
}

// Int64ToStr 转换 int64 为 string
func Int64ToStr(val int64) string {
	return strconv.FormatInt(val, 10)
}

// StrToFloat64 获取查询参数并转换为 float64，如果失败则返回默认值
func StrToFloat64(strVal string) (float64, error) {
	val, err := strconv.ParseFloat(strVal, 64)
	if err != nil {
		return 0, err
	}
	return val, nil
}

// Float64ToStr 转换 float64 为 string
func Float64ToStr(val float64) string {
	return strconv.FormatFloat(val, 'f', -1, 64)
}

// StrToBool 获取查询参数并转换为 bool，如果失败则返回默认值
func StrToBool(strVal string) (bool, error) {
	val, err := strconv.ParseBool(strVal)
	if err != nil {
		return false, err
	}
	return val, nil
}

// BoolToStr 转换 bool 为 string
func BoolToStr(val bool) string {
	return strconv.FormatBool(val)
}
