package cb

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-mego/mego"
)

var (
	// ErrOpenState 表示斷路器處於開啟狀態，所有請求都被拒絕。
	ErrOpenState = errors.New("circuitbreaker: the circuit breaker is open")
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

const (
	// StateClosed 表示斷路器處於關閉狀態，請求可正常通過。
	StateClosed State = iota
	// StateHalfOpen 表示斷路器處於半開放狀態，請求可正常通過但任何錯誤都會導致斷路器關閉。
	StateHalfOpen
	// StateOpen 表示斷路器處於開啟狀態，所有請求均被拒絕。
	StateOpen
)

// Counts 是斷路器的計數狀態。
type Counts struct {
	// TotalSuccesses 是總共的成功次數。
	TotalSuccesses int
	// TotalFailures 是總共的失敗次數。
	TotalFailures int
	// ConsecutiveSuccesses 是連續的成功次數。
	ConsecutiveSuccesses int
	// ConsecutiveFailures 是連續的失敗次數。
	ConsecutiveFailures int
}

// Options 是斷路器的選項設置。
type Options struct {
	// Name 是斷路器的名稱。
	Name string
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

// State 是斷路器狀態。
type State int

// String 能夠將一個斷路器狀態轉會成人類可讀的文字（`closed`、`half-open`、`open`）。
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateHalfOpen:
		return "half-open"
	case StateOpen:
		return "open"
	}
	return ""
}

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
		name:         o.Name,
		options:      o,
		counts:       &Counts{},
		state:        StateClosed,
		lastInterval: time.Now(),
	}
	return func(c *mego.Context) {
		// 如果斷路器處於開啟狀態。
		if b.State() == StateOpen {
			// 要是上次失敗的時間已經超過我們所設定的逾時時間，
			// 那麼就給斷路器一次機會，回到半開放狀態。
			if time.Since(b.lastFailure) >= b.options.Timeout {
				b.state = StateHalfOpen
			}
		}
		// 如果斷路器處於關閉狀態。
		if b.State() == StateClosed {
			// 要是上次失敗的時間已經超過了我們所設定的週期時間，
			// 那麼就重設斷路器的所有資訊，假裝先前的錯誤不曾發生過。
			if time.Since(b.lastInterval) >= b.options.Interval {
				b.reset()
			}
			// 呼叫過路函式，讓開發者決定是否要開啟斷路器。
			if b.options.OnTrip(c, *b.counts) {
				b.state = StateOpen
			}
		}
		// 如果經過前面那些條件，斷路器還是開啟的話就回傳 HTTP 內部伺服器錯誤狀態碼。
		if b.State() == StateOpen {
			c.AbortWithError(http.StatusServiceUnavailable, ErrOpenState)
			return
		}

		c.Map(b)
		c.Next()

		// 當整個路由函式鏈結束時。
		defer func() {
			// 檢查此路由回傳的狀態碼是否在錯誤狀態碼列表內。
			for _, v := range b.options.FailureStatuses {
				// 如果回應的狀態碼屬於錯誤狀態碼，就像斷路器表明此次請求失敗並計次遞加。
				if c.Writer.Status() == v {
					b.fail()
					return
				}
			}
			// 不然就算此次請求成功。
			b.success()
		}()
	}
}

// tripper 是預設的斷路器裝置，會在連續失敗 5 次後啟動斷路器。
func tripper(ctx *mego.Context, counts Counts) bool {
	return counts.ConsecutiveFailures >= 5
}

// Breaker 是一個斷路器。
type Breaker struct {
	// name 是這個斷路器的名稱。
	name string
	// state 是斷路器目前的狀態。
	state State
	// lastFailure 是最後一次發生失敗的時間，用以比對經過了多久。
	lastFailure time.Time
	// lastInterval 是最後一次的週期時間。
	lastInterval time.Time
	// options 是斷路器的選項。
	options *Options
	// counts 是斷路器的計數器。
	counts *Counts
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

// Counts 能夠取得斷路器的計數狀態。
func (b *Breaker) Counts() Counts {
	return *b.counts
}

// fail 會追加失敗次數。
func (b *Breaker) fail() {
	b.lastFailure = time.Now()
	b.counts.TotalFailures++
	b.counts.ConsecutiveFailures++
	b.counts.ConsecutiveSuccesses = 0
	// 如果失敗的時候，斷路器處於半開放狀態，那麼就回歸開放狀態拒絕所有請求。
	if b.State() == StateHalfOpen {
		b.state = StateOpen
	}
}

// success 會追加成功次數。
func (b *Breaker) success() {
	b.counts.TotalSuccesses++
	b.counts.ConsecutiveSuccesses++
	b.counts.ConsecutiveFailures = 0
	// 如果成功的時候，斷路器處於半開放狀態，那麼就重設斷路器的所有資訊。
	if b.State() == StateHalfOpen {
		b.reset()
	}
}

// reset 會重設斷路器的資訊。
func (b *Breaker) reset() {
	b.counts = &Counts{}
	b.lastInterval = time.Now()
	b.state = StateClosed
}
