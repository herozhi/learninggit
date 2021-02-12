package gls

import (
	"unsafe"
)

type Storage interface {
	GetData(k string) interface{}
	SetData(k string, i interface{})
	Clear()
	ToUnsafePointer() unsafe.Pointer
}

type storage struct {
	data map[string]interface{}
}

func FromUnsafePointer(p unsafe.Pointer) Storage {
	return (*storage)(p)
}

func (s *storage) ToUnsafePointer() unsafe.Pointer {
	return unsafe.Pointer(s)
}

func NewStorage() Storage {
	return &storage{make(map[string]interface{})}
}

func (s *storage) GetData(k string) interface{} {
	if s != nil {
		if i, ok := s.data[k]; ok {
			return i
		}
	}
	return nil
}

func (s *storage) SetData(k string, i interface{}) {
	if s != nil {
		s.data[k] = i
	}
}

func (s *storage) Clear() {
	if s != nil {
		s.data = make(map[string]interface{})
	}
}
