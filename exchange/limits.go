package exchange

// type MyError string

// const (
// 	ErrOrderRateLimits MyError = "error_order_rate_limits" // 下单限速
// 	ErrIpRateLimits    MyError = "error_ip_rate_limits"    // IP限速
// )

// func (e MyError) Error() string {
// 	return string(e)
// }

type RateLimit struct {
	msg        string
	retryAfter int
}

func (r *RateLimit) Error() string {
	return r.msg
}

func (r *RateLimit) RetryAfter() int {
	return r.retryAfter
}

func NewRateLimit(msg string, retryAfter int) *RateLimit {
	return &RateLimit{
		msg:        msg,
		retryAfter: retryAfter,
	}
}
