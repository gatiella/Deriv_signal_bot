package signals

// Momentum tracks short-term tick-to-tick direction over a rolling window,
// as a descriptive lean indicator for Rise/Fall.
//
// HONESTY NOTE: for a pure random-walk price process (which is what Deriv's
// synthetic indices are designed to approximate), there is no persistent
// short-term momentum to exploit - a run of upward ticks is not evidence the
// next tick is more likely to go up OR down. This indicator reports what
// just happened, which is sometimes useful context, but treat any apparent
// "edge" here as something to verify with backtest.Run against real history,
// not something to assume.
type Momentum struct {
	window     []int8 // +1 up, -1 down, 0 unchanged
	capacity   int
	pos        int
	filled     int
	lastQuote  float64
	haveLast   bool
	ups, downs int
}

// NewMomentum creates a tracker with the given rolling window size (in ticks).
func NewMomentum(windowSize int) *Momentum {
	return &Momentum{
		window:   make([]int8, windowSize),
		capacity: windowSize,
	}
}

// Push records a new quote, comparing it to the previous one.
func (m *Momentum) Push(quote float64) {
	if !m.haveLast {
		m.lastQuote = quote
		m.haveLast = true
		return
	}
	var dir int8
	switch {
	case quote > m.lastQuote:
		dir = 1
	case quote < m.lastQuote:
		dir = -1
	default:
		dir = 0
	}
	m.lastQuote = quote

	if m.filled == m.capacity {
		old := m.window[m.pos]
		m.adjustCounts(old, -1)
	} else {
		m.filled++
	}
	m.window[m.pos] = dir
	m.adjustCounts(dir, 1)
	m.pos = (m.pos + 1) % m.capacity
}

func (m *Momentum) adjustCounts(dir int8, delta int) {
	switch dir {
	case 1:
		m.ups += delta
	case -1:
		m.downs += delta
	}
}

// Ready reports whether the window has enough samples to be meaningful.
func (m *Momentum) Ready() bool {
	return m.filled >= m.capacity/2
}

// Bias returns the fraction (0-1) of directional ticks (ignoring unchanged)
// that moved up, and the fraction that moved down. They sum to <= 1.
func (m *Momentum) Bias() (upFrac, downFrac float64) {
	total := m.ups + m.downs
	if total == 0 {
		return 0, 0
	}
	return float64(m.ups) / float64(total), float64(m.downs) / float64(total)
}

// WindowSize returns the configured capacity.
func (m *Momentum) WindowSize() int { return m.capacity }
