package signals

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// EngineConfig tunes how sensitive the engine is. There is no "correct"
// setting - these are starting points, meant to be adjusted once you've run
// backtest.Run and seen how a given threshold actually performs historically.
type EngineConfig struct {
	DigitWindowSize    int
	DigitThresholdPct  float64 // e.g. 4.0 means a digit at >=14% or <=6% counts as hot/cold
	MomentumWindowSize int
	MomentumThreshold  float64 // e.g. 0.65 means 65%+ of directional ticks one way counts as a lean
}

type symbolState struct {
	digits   *DigitStats
	momentum *Momentum

	// edge-detection so we only emit a Signal when something newly crosses
	// the threshold, instead of firing on every single tick
	hotDigits  [10]bool
	coldDigits [10]bool
	evenSkewed bool
	oddSkewed  bool
	upLean     bool
	downLean   bool
}

// Engine consumes ticks for multiple symbols and produces Snapshots (for the
// live dashboard display) and Signals (discrete, edge-triggered flags).
type Engine struct {
	cfg EngineConfig

	mu     sync.Mutex
	states map[string]*symbolState

	onSignal   func(Signal)
	onSnapshot func(Snapshot)
}

// NewEngine builds an Engine. onSignal and onSnapshot are called
// synchronously from Feed - keep them fast (e.g. push to a channel) or wrap
// them yourself if they need to do I/O.
func NewEngine(cfg EngineConfig, onSignal func(Signal), onSnapshot func(Snapshot)) *Engine {
	return &Engine{
		cfg:        cfg,
		states:     make(map[string]*symbolState),
		onSignal:   onSignal,
		onSnapshot: onSnapshot,
	}
}

func (e *Engine) stateFor(symbol string) *symbolState {
	st, ok := e.states[symbol]
	if !ok {
		st = &symbolState{
			digits:   NewDigitStats(e.cfg.DigitWindowSize),
			momentum: NewMomentum(e.cfg.MomentumWindowSize),
		}
		e.states[symbol] = st
	}
	return st
}

// Feed processes one new tick for a symbol.
func (e *Engine) Feed(symbol string, quote float64, pipSize int) {
	e.mu.Lock()
	defer e.mu.Unlock()

	st := e.stateFor(symbol)
	digit := LastDigit(quote, pipSize)
	st.digits.Push(digit)
	st.momentum.Push(quote)

	if st.digits.Ready() {
		e.checkDigitEdges(symbol, st)
	}
	if st.momentum.Ready() {
		e.checkMomentumEdges(symbol, st)
	}

	e.emitSnapshot(symbol, st, quote)
}

func (e *Engine) checkDigitEdges(symbol string, st *symbolState) {
	freq := st.digits.Frequencies()
	baseline := 10.0

	for digit := 0; digit < 10; digit++ {
		dev := freq[digit] - baseline
		isHot := dev >= e.cfg.DigitThresholdPct
		isCold := -dev >= e.cfg.DigitThresholdPct

		if isHot && !st.hotDigits[digit] {
			e.onSignal(Signal{
				Symbol:       symbol,
				ContractType: DigitOver,
				Detail:       fmt.Sprintf("digit %d is running hot: %.1f%% of last %d ticks (baseline 10%%)", digit, freq[digit], st.digits.WindowSize()),
				Strength:     dev,
				WindowSize:   st.digits.WindowSize(),
				CreatedAt:    time.Now(),
			})
		}
		if isCold && !st.coldDigits[digit] {
			e.onSignal(Signal{
				Symbol:       symbol,
				ContractType: DigitUnder,
				Detail:       fmt.Sprintf("digit %d is running cold: %.1f%% of last %d ticks (baseline 10%%)", digit, freq[digit], st.digits.WindowSize()),
				Strength:     dev,
				WindowSize:   st.digits.WindowSize(),
				CreatedAt:    time.Now(),
			})
		}
		st.hotDigits[digit] = isHot
		st.coldDigits[digit] = isCold
	}

	evenPct, oddPct := st.digits.EvenOddSplit()
	evenDev := evenPct - 50
	oddDev := oddPct - 50
	evenSkewed := math.Abs(evenDev) >= e.cfg.DigitThresholdPct && evenDev > 0
	oddSkewed := math.Abs(oddDev) >= e.cfg.DigitThresholdPct && oddDev > 0

	if evenSkewed && !st.evenSkewed {
		e.onSignal(Signal{
			Symbol:       symbol,
			ContractType: DigitEven,
			Detail:       fmt.Sprintf("even digits at %.1f%% of last %d ticks (baseline 50%%)", evenPct, st.digits.WindowSize()),
			Strength:     evenDev,
			WindowSize:   st.digits.WindowSize(),
			CreatedAt:    time.Now(),
		})
	}
	if oddSkewed && !st.oddSkewed {
		e.onSignal(Signal{
			Symbol:       symbol,
			ContractType: DigitOdd,
			Detail:       fmt.Sprintf("odd digits at %.1f%% of last %d ticks (baseline 50%%)", oddPct, st.digits.WindowSize()),
			Strength:     oddDev,
			WindowSize:   st.digits.WindowSize(),
			CreatedAt:    time.Now(),
		})
	}
	st.evenSkewed = evenSkewed
	st.oddSkewed = oddSkewed
}

func (e *Engine) checkMomentumEdges(symbol string, st *symbolState) {
	up, down := st.momentum.Bias()
	upLean := up >= e.cfg.MomentumThreshold
	downLean := down >= e.cfg.MomentumThreshold

	if upLean && !st.upLean {
		e.onSignal(Signal{
			Symbol:       symbol,
			ContractType: Rise,
			Detail:       fmt.Sprintf("%.0f%% of the last %d ticks moved up", up*100, st.momentum.WindowSize()),
			Strength:     up,
			WindowSize:   st.momentum.WindowSize(),
			CreatedAt:    time.Now(),
		})
	}
	if downLean && !st.downLean {
		e.onSignal(Signal{
			Symbol:       symbol,
			ContractType: Fall,
			Detail:       fmt.Sprintf("%.0f%% of the last %d ticks moved down", down*100, st.momentum.WindowSize()),
			Strength:     down,
			WindowSize:   st.momentum.WindowSize(),
			CreatedAt:    time.Now(),
		})
	}
	st.upLean = upLean
	st.downLean = downLean
}

func (e *Engine) emitSnapshot(symbol string, st *symbolState, lastQuote float64) {
	freq := st.digits.Frequencies()
	digits := make([]SnapshotDigit, 10)
	for i := 0; i < 10; i++ {
		digits[i] = SnapshotDigit{
			Digit:     i,
			Frequency: freq[i],
			Hot:       st.hotDigits[i],
			Cold:      st.coldDigits[i],
		}
	}
	evenPct, oddPct := st.digits.EvenOddSplit()
	up, down := st.momentum.Bias()

	e.onSnapshot(Snapshot{
		Symbol:      symbol,
		Digits:      digits,
		EvenPct:     evenPct,
		OddPct:      oddPct,
		UpFrac:      up,
		DownFrac:    down,
		LastQuote:   lastQuote,
		UpdatedAt:   time.Now(),
		DigitWindow: st.digits.WindowSize(),
		TickWindow:  st.momentum.WindowSize(),
	})
}
