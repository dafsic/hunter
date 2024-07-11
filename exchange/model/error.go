package model

type MyError string

const (
	ErrOrderRateLimits MyError = "error_order_rate_limits" // 下单限速
	ErrIpRateLimits    MyError = "error_ip_rate_limits"    // IP限速
)

func (e MyError) Error() string {
	return string(e)
}
