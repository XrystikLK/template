package positions

import "time"

type realTicker struct {
	ticker *time.Ticker
}

func NewRealTicker(d time.Duration) Ticker {
	return &realTicker{ticker: time.NewTicker(d)}
}

func (r *realTicker) Chan() <-chan time.Time {
	return r.ticker.C
}

func (r *realTicker) Stop() {
	r.ticker.Stop()
}
