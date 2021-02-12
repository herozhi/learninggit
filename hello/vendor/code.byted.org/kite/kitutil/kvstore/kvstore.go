package kvstore

// KVStorer .
type KVStorer interface {
	Get(key string) (string, error)
	// create this key if not exist
	GetOrCreate(key, defaultVal string) (string, error)
}
