package circuitbreaker

import (
	"sync"
	"time"
)

// panel manages a batch of circuitbreakers
type panel struct {
	breakers       sync.Map
	defaultOptions Options
	changeHandler  PanelStateChangeHandler
	ticker         *time.Ticker
}

// NewPanel .
func NewPanel(changeHandler PanelStateChangeHandler,
	defaultOptions Options) (Panel, error) {
	if defaultOptions.BucketTime <= 0 {
		defaultOptions.BucketTime = defaultBucketTime
	}

	if defaultOptions.BucketNums <= 0 {
		defaultOptions.BucketNums = defaultBucketNums
	}

	if defaultOptions.CoolingTimeout <= 0 {
		defaultOptions.CoolingTimeout = defaultCoolingTimeout
	}

	if defaultOptions.DetectTimeout <= 0 {
		defaultOptions.DetectTimeout = defaultDetectTimeout
	}
	_, err := newBreaker(defaultOptions)
	if err != nil {
		return nil, err
	}
	p := &panel{
		breakers:       sync.Map{},
		defaultOptions: defaultOptions,
		changeHandler:  changeHandler,
		ticker:         time.NewTicker(defaultOptions.BucketTime),
	}
	go p.tick()
	return p, nil
}

// getBreaker .
func (p *panel) getBreaker(key string) *breaker {
	cb, ok := p.breakers.Load(key)
	if ok {
		return cb.(*breaker)
	}

	op := p.defaultOptions
	if p.changeHandler != nil {
		op.BreakerStateChangeHandler = func(oldState, newState State, m Metricer) {
			p.changeHandler(key, oldState, newState, m)
		}
	}
	ncb, _ := newBreaker(op)
	cb, ok = p.breakers.LoadOrStore(key, ncb)
	return cb.(*breaker)
}

// RemoveBreaker .
func (p *panel) RemoveBreaker(key string) {
	p.breakers.Delete(key)
}

// DumpBreakers .
func (p *panel) DumpBreakers() map[string]Breaker {
	breakers := make(map[string]Breaker)
	p.breakers.Range(func(key, value interface{}) bool {
		breakers[key.(string)] = value.(*breaker)
		return true
	})
	return breakers
}

// Succeed .
func (p *panel) Succeed(key string) {
	p.getBreaker(key).Succeed()
}

// Fail .
func (p *panel) Fail(key string) {
	p.getBreaker(key).Fail()
}

// FailWithTrip .
func (p *panel) FailWithTrip(key string, f TripFunc) {
	p.getBreaker(key).FailWithTrip(f)
}

// Timeout .
func (p *panel) Timeout(key string) {
	p.getBreaker(key).Timeout()
}

// TimeoutWithTrip .
func (p *panel) TimeoutWithTrip(key string, f TripFunc) {
	p.getBreaker(key).TimeoutWithTrip(f)
}

// IsAllowed .
func (p *panel) IsAllowed(key string) bool {
	return p.getBreaker(key).IsAllowed()
}

func (p *panel) Close() {
	p.ticker.Stop()
}

// tick .
func (p *panel) tick() {
	for range p.ticker.C {
		p.breakers.Range(func(key, value interface{}) bool {
			if b, ok := value.(*breaker); ok {
				b.metricer.tick()
			}
			return true
		})
	}
}
