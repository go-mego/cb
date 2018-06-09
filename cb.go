package cb

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-mego/mego"
)

var (
	// ErrTooManyRequests 表示斷路器於半開放狀態但有太多請求湧入而暫時被拒。
	ErrTooManyRequests = errors.New("breaker: too many requests")
	// ErrOpenState 表示斷路器處於開啟狀態，所有請求都被拒絕。
	ErrOpenState = errors.New("breaker: the circuit breaker is open")
	// DefaultFailureStatuses 是預設的錯誤狀態碼清單，可用於自動錯誤偵測上。
	DefaultFailureStatuses = []int{
		http.StatusInternalServerError,   // 500
		http.StatusBadGateway,            // 502
		http.StatusServiceUnavailable,    // 503
		http.StatusGatewayTimeout,        // 504
		http.StatusVariantAlsoNegotiates, // 506
		http.StatusInsufficientStorage,   // 507
	}
	// EmptyFailureStatuses 是一個空的錯誤狀態碼清單。
	EmptyFailureStatuses = []int{}
)

// State 是斷路器狀態。
type State int

const (
	// StateClosed 表示斷路器處於關閉狀態，請求可正常通過。
	StateClosed State = iota
	// StateHalfOpen 表示斷路器處於半開放狀態，請求可正常通過但任何錯誤都會導致斷路器關閉。
	StateHalfOpen
	// StateOpen 表示斷路器處於開啟狀態，所有請求均被拒絕。
	StateOpen
)

// New 會建立一個斷路器。
func New(options ...*Options) mego.HandlerFunc {
	o := &Options{
		FailureStatuses: DefaultFailureStatuses,
	}
	if len(options) > 0 {
		o = options[0]
	}
	if o.Name == "" {
		o.Name = "CircuitBreaker"
	}
	if o.MaxRequests == 0 {
		o.MaxRequests = 1
	}
	if o.Timeout.Seconds() == 0 {
		o.Timeout = time.Second * 60
	}
	if o.Interval.Seconds() == 0 {
		o.Interval = time.Second * 60
	}
	if o.OnTrip == nil {
		o.OnTrip = tripper
	}
	b := &Breaker{
		name:    o.Name,
		options: o,
		counts:  &Counts{},
		state:   StateClosed,
	}
	return func(c *mego.Context) {
		/*b.check()
		if b.State() == StateOpen {
			c.AbortWithError(http.StatusServiceUnavailable, ErrOpenState)
			return
		}
		v := b.options.OnTrip(c, *b.counts)
		if v {
			b.Open()
		}
		if b.State() == StateOpen {
			c.AbortWithError(http.StatusServiceUnavailable, ErrOpenState)
			return
		}*/
		c.Map(b)
		c.Next()
		defer func() {
			s := c.Writer.Status()
			for _, v := range b.options.FailureStatuses {
				if v == s {
					b.Fail()
					return
				}
			}
		}()
	}
}

// tripper 是預設的斷路器裝置，會在連續失敗 5 次後啟動斷路器。
func tripper(ctx *mego.Context, counts Counts) bool {
	fmt.Printf("%+v", counts)
	return counts.ConsecutiveFailures > 5
}

// Counts 是斷路器的計數狀態。
type Counts struct {
	//
	Requests int
	// TotalSuccesses 是總共的成功次數。
	TotalSuccesses int
	// TotalFailures 是總共的失敗次數。
	TotalFailures int
	// ConsecutiveSuccesses 是連續的成功次數。
	ConsecutiveSuccesses int
	// ConsecutiveFailures 是連續的失敗次數。
	ConsecutiveFailures int
}

// success 會追加成功次數。
func (c *Counts) success() {
	c.TotalSuccesses++
	c.ConsecutiveSuccesses++
	c.ConsecutiveFailures = 0
}

// fail 會追加失敗次數。
func (c *Counts) fail() {
	c.TotalFailures++
	c.ConsecutiveFailures++
	c.ConsecutiveSuccesses = 0
}

// Options 是斷路器的選項設置。
type Options struct {
	// Name 是斷路器的名稱。
	Name string
	// MaxRequests 是斷路器在半開放狀態可允許的最大請求數，當此值為 0 會採用預設值（1）。
	MaxRequests int
	// FailureStatuses 是自動失敗 HTTP 狀態碼，當回應的狀態碼於此清單內會自動視為失敗而計次。
	// `DefaultFailureStatuses` 是預設的 5xx 伺服器錯誤碼清單，若不想使用此功能可傳入 `EmptyFailureStatuses` 空狀態碼清單。
	FailureStatuses []int
	// Interval 是斷路器的循環週期。在斷路器復歸時經過此秒數後會重設整個斷路器資訊。
	// 預設為 60 秒。
	Interval time.Duration
	// Timeout 是斷路器在開啟後的逾時秒數，經過此時間後斷路器會成為半開放狀態。
	// 預設為 60 秒。
	Timeout time.Duration
	// OnTrip 會在每次經過斷路器時所觸發，此函式會接收上下文建構體與目前的計次狀態。
	// 當此函式回傳 `true` 時，斷路器就會被開啟而拒絕接下來的請求。
	// 此函式預設為連續失敗 5 次就斷路。
	OnTrip func(ctx *mego.Context, counts Counts) bool
	// OnStateChange 會在斷路器的狀態變更時呼叫。
	OnStateChange func(name string, from State, to State)
}

// Breaker 是一個斷路器。
type Breaker struct {
	// name 是這個斷路器的名稱。
	name string
	// state 是斷路器目前的狀態。
	state State
	// lastFailure 是最後一次發生失敗的時間，用以比對經過了多久。
	lastFailure time.Time
	// options 是斷路器的選項。
	options *Options
	// counts 是斷路器的計數器。
	counts *Counts
}

// Execute 能夠在斷路器中執行函式，
// 當該函式回傳的 `error` 並非 `nil` 值時，錯誤計次會遞加。
func (b *Breaker) Execute(fn func() (value interface{}, err error)) (value interface{}, err error) {
	value, err = fn()
	if err != nil {
		b.counts.fail()
		b.lastFailure = time.Now()
		return
	}
	b.counts.success()
	return
}

// Fail 表示此次行動失敗，錯誤計次會遞加。
func (b *Breaker) Fail() {
	b.counts.fail()
	b.lastFailure = time.Now()
}

// Open 會直接開啟斷路器拒絕接下來的請求。
func (b *Breaker) Open() {
	b.state = StateOpen
}

// Close 會直接關閉斷路器並允許接下來的請求。
func (b *Breaker) Close() {
	b.state = StateClosed
}

// Name 能夠取得斷路器的名稱。
func (b *Breaker) Name() string {
	return b.name
}

// State 能夠取得斷路器的目前狀態。
func (b *Breaker) State() State {
	return b.state
}

// check 會在每次經過斷路器時呼叫，這用以基於現在的資訊來作為是否斷路的依據。
func (b *Breaker) check() {
	duration := time.Since(b.lastFailure)
	switch b.state {
	case StateOpen:
		if duration >= b.options.Timeout {
			b.state = StateHalfOpen
		}
	case StateHalfOpen:
		/*if b.counts.ConsecutiveFailures == 0 {
			b.reset()
		} else {
			b.state = StateOpen
		}*/
	case StateClosed:
		if duration >= b.options.Interval {
			b.reset()
		}
	}
}

// reset 會重設斷路器的資訊。
func (b *Breaker) reset() {
	b.counts = &Counts{}
	b.state = StateClosed
}
