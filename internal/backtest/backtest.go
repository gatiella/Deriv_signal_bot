// Package backtest answers the question this whole project exists to answer
// honestly: does watching digit frequency or tick momentum actually predict
// anything, or is it noise? It replays REAL historical ticks from Deriv
// through the exact same trackers the live engine uses, simulates placing
// the "obvious" bet every time a threshold is crossed, and reports the
// realized win rate against the true theoretical baseline for that contract.
//
// This is deliberately built to test the two ways people commonly reason
// about these strategies on social media:
//   - "due" / gambler's-fallacy reasoning: a cold digit is due to appear,
//     a losing streak is about to reverse
//   - "streak" / momentum reasoning: a hot digit or trending price will
//     keep going
//
// Expect win rates close to the baseline. That's not a bug in the
// backtester - it's the actual answer for a fair random process, and it's
// the honest thing to show before anyone risks money on a rule like this.
package backtest

import (
	"fmt"
	"math"

	"github.com/gatiella/deriv-signal-bot/internal/deriv"
	"github.com/gatiella/deriv-signal-bot/internal/signals"
)

// Config controls sample size and the same thresholds the live engine uses,
// so you can test the exact settings you're running live.
type Config struct {
	Symbol             string
	SampleSize         int // number of historical ticks to replay (Deriv caps ticks_history, typically ~5000)
	DigitWindowSize    int
	DigitThresholdPct  float64
	MomentumWindowSize int
	MomentumThreshold  float64
}

// RuleResult reports how one specific betting rule performed against real history.
type RuleResult struct {
	Rule        string  `json:"rule"`
	Description string  `json:"description"`
	Triggers    int     `json:"triggers"` // how many times this rule's condition fired
	Wins        int     `json:"wins"`
	WinRatePct  float64 `json:"win_rate_pct"`
	BaselinePct float64 `json:"baseline_pct"` // theoretical win rate for a fair/random process
	StdErrPct   float64 `json:"std_err_pct"`  // binomial standard error, for eyeballing significance
	Verdict     string  `json:"verdict"`
}

// Report is the full output of a backtest run.
type Report struct {
	Symbol     string       `json:"symbol"`
	SampleSize int          `json:"sample_size"`
	Results    []RuleResult `json:"results"`
}

// Run fetches historical ticks for cfg.Symbol and replays them through
// fresh digit/momentum trackers, scoring six rules along the way. client
// should be an unauthenticated (or authenticated - doesn't matter) Deriv
// connection; ticks_history is public data.
func Run(client *deriv.Client, cfg Config) (*Report, error) {
	pipSize, err := lookupPipSize(client, cfg.Symbol)
	if err != nil {
		return nil, fmt.Errorf("look up pip size for %s: %w", cfg.Symbol, err)
	}

	hist, err := client.TicksHistoryTicks(cfg.Symbol, cfg.SampleSize)
	if err != nil {
		return nil, fmt.Errorf("fetch tick history: %w", err)
	}
	if len(hist.Prices) < cfg.DigitWindowSize+2 {
		return nil, fmt.Errorf("not enough historical ticks returned (%d) to fill a window of %d", len(hist.Prices), cfg.DigitWindowSize)
	}

	sc := newScorer(cfg)
	digitStats := signals.NewDigitStats(cfg.DigitWindowSize)
	momentum := signals.NewMomentum(cfg.MomentumWindowSize)

	var prevHot, prevCold [10]bool
	var prevEvenSkew, prevOddSkew, prevUpLean, prevDownLean bool

	for i, price := range hist.Prices {
		digit := signals.LastDigit(price, pipSize)
		digitStats.Push(digit)
		momentum.Push(price)

		hasNext := i+1 < len(hist.Prices)
		var nextDigit int
		var nextUp, nextDown bool
		if hasNext {
			nextDigit = signals.LastDigit(hist.Prices[i+1], pipSize)
			if hist.Prices[i+1] > price {
				nextUp = true
			} else if hist.Prices[i+1] < price {
				nextDown = true
			}
		}

		if digitStats.Ready() && hasNext {
			freq := digitStats.Frequencies()
			for d := 0; d < 10; d++ {
				dev := freq[d] - 10.0
				isHot := dev >= cfg.DigitThresholdPct
				isCold := -dev >= cfg.DigitThresholdPct

				if isHot && !prevHot[d] {
					sc.record("digit_streak", nextDigit == d)
				}
				if isCold && !prevCold[d] {
					sc.record("digit_due", nextDigit == d)
				}
				prevHot[d] = isHot
				prevCold[d] = isCold
			}

			evenPct, oddPct := digitStats.EvenOddSplit()
			evenSkew := evenPct-50 >= cfg.DigitThresholdPct
			oddSkew := oddPct-50 >= cfg.DigitThresholdPct
			nextIsEven := nextDigit%2 == 0
			if evenSkew && !prevEvenSkew {
				sc.record("evenodd_streak", nextIsEven)
				sc.record("evenodd_due", !nextIsEven)
			}
			if oddSkew && !prevOddSkew {
				sc.record("evenodd_streak", !nextIsEven)
				sc.record("evenodd_due", nextIsEven)
			}
			prevEvenSkew, prevOddSkew = evenSkew, oddSkew
		}

		if momentum.Ready() && hasNext && (nextUp || nextDown) {
			up, down := momentum.Bias()
			upLean := up >= cfg.MomentumThreshold
			downLean := down >= cfg.MomentumThreshold
			if upLean && !prevUpLean {
				sc.record("momentum_continuation", nextUp)
				sc.record("momentum_reversion", nextDown)
			}
			if downLean && !prevDownLean {
				sc.record("momentum_continuation", nextDown)
				sc.record("momentum_reversion", nextUp)
			}
			prevUpLean, prevDownLean = upLean, downLean
		}
	}

	return &Report{
		Symbol:     cfg.Symbol,
		SampleSize: len(hist.Prices),
		Results:    sc.results(),
	}, nil
}

func lookupPipSize(client *deriv.Client, symbol string) (int, error) {
	list, err := client.ActiveSymbols()
	if err != nil {
		return 0, err
	}
	for _, s := range list {
		if s.Symbol == symbol && s.Pip > 0 {
			return int(math.Round(-math.Log10(s.Pip))), nil
		}
	}
	return 0, fmt.Errorf("symbol %s not found in active_symbols", symbol)
}

// scorer accumulates trigger/win counts per named rule.
type scorer struct {
	cfg      Config
	triggers map[string]int
	wins     map[string]int
}

func newScorer(cfg Config) *scorer {
	return &scorer{
		cfg:      cfg,
		triggers: make(map[string]int),
		wins:     make(map[string]int),
	}
}

func (s *scorer) record(rule string, win bool) {
	s.triggers[rule]++
	if win {
		s.wins[rule]++
	}
}

type ruleMeta struct {
	name        string
	description string
	baseline    float64
}

var ruleDefs = []ruleMeta{
	{"digit_due", "bet the next digit repeats a digit that just went cold (gambler's-fallacy 'it's due')", 10.0},
	{"digit_streak", "bet the next digit repeats a digit that just went hot (momentum/'streak continues')", 10.0},
	{"evenodd_due", "bet the next digit flips parity after one side skews (reversion)", 50.0},
	{"evenodd_streak", "bet the next digit keeps the same parity that's currently skewed (continuation)", 50.0},
	{"momentum_continuation", "bet the next tick continues the recent directional lean (Rise/Fall)", 50.0},
	{"momentum_reversion", "bet the next tick reverses the recent directional lean (Rise/Fall)", 50.0},
}

func (s *scorer) results() []RuleResult {
	out := make([]RuleResult, 0, len(ruleDefs))
	for _, def := range ruleDefs {
		triggers := s.triggers[def.name]
		wins := s.wins[def.name]
		var winRate, stdErr float64
		verdict := "not enough triggers in this sample to judge"
		if triggers > 0 {
			winRate = float64(wins) / float64(triggers) * 100
			p := def.baseline / 100
			stdErr = math.Sqrt(p*(1-p)/float64(triggers)) * 100
			diff := math.Abs(winRate - def.baseline)
			switch {
			case triggers < 20:
				verdict = "too few triggers to judge - widen the sample"
			case diff <= 2*stdErr:
				verdict = "no meaningful edge - within normal random variation of baseline"
			default:
				verdict = "outside normal variation - worth a larger sample before trusting this, could still be noise"
			}
		}
		out = append(out, RuleResult{
			Rule:        def.name,
			Description: def.description,
			Triggers:    triggers,
			Wins:        wins,
			WinRatePct:  round1(winRate),
			BaselinePct: def.baseline,
			StdErrPct:   round1(stdErr),
			Verdict:     verdict,
		})
	}
	return out
}

func round1(f float64) float64 {
	return math.Round(f*10) / 10
}
