package mysql

import (
	"fmt"

	"code.byted.org/gopkg/consul"
)

var consulAddr string
var ipv4 string
var ipv6 string
var option string
var max = 10
var index = 0

func consulGet(service string) (consul.Endpoints, error) {
	eps, err := consul.LookupName(service)
	if err != nil {
		return nil, err
	}
	if len(eps) == 0 {
		return consul.Endpoints{}, fmt.Errorf("got empty result")
	}
	return eps, nil
}
