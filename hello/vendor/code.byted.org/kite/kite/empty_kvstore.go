package kite

type emptyStorer struct{}

func newEmptyStorer() *emptyStorer {
	return &emptyStorer{}
}

func (e *emptyStorer) Get(key string) (string, error) {
	return "0", nil
}

func (e *emptyStorer) GetOrCreate(key, val string) (string, error) {
	return val, nil
}

