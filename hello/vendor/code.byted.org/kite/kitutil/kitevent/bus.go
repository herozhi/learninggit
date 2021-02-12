package kitevent

import (
	"fmt"
	"sync"
)

type Handler func(event *KitEvent)

type EventBus interface {
	Watch(event string, handler Handler)
	Unwatch(event string, handler Handler)
	Dispatch(event *KitEvent)
}

func NewEventBus() EventBus {
	return &eventBus{}
}

type eventBus struct {
	callbacks sync.Map
}

func (eb *eventBus) Watch(event string, handler Handler) {
	var handlers []Handler
	if actual, ok := eb.callbacks.Load(event); ok {
		handlers = actual.([]Handler)
	}
	handlers = append(handlers, handler)
	eb.callbacks.Store(event, handlers)
}

func (eb *eventBus) Unwatch(event string, handler Handler) {
	var filtered []Handler
	// In go, functions are not comparable, so we use fmt.Sprint to relect their names for comparation.
	target := fmt.Sprint(handler)
	if actual, ok := eb.callbacks.Load(event); ok {
		for _, h := range actual.([]Handler) {
			if fmt.Sprint(h) != target {
				filtered = append(filtered, h)
			}
		}
	}
	eb.callbacks.Store(event, filtered)
}

func (eb *eventBus) Dispatch(event *KitEvent) {
	if actual, ok := eb.callbacks.Load(event.Name); ok {
		for _, h := range actual.([]Handler) {
			go h(event)
		}
	}
}
