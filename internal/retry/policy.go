package retry

type Policy struct {
	MaxAttempts int
	Backoff     string
	Delay       string
}
