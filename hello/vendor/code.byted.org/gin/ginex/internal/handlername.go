package internal

import (
	"reflect"

	"github.com/gin-gonic/gin"
)

var (
	handlerNames = make(map[uintptr]string)
)

func SetHandlerName(handler gin.HandlerFunc, name string) {
	handlerNames[getFuncAddr(handler)] = name
}

func GetHandlerName(handler gin.HandlerFunc) string {
	return handlerNames[getFuncAddr(handler)]
}

func getFuncAddr(v interface{}) uintptr {
	return reflect.ValueOf(reflect.ValueOf(v)).Field(1).Pointer()
}
