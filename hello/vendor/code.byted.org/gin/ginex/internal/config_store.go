package internal

import (
	"fmt"

	"code.byted.org/gin/ginex/configstorer"
)

func InitConfigStorer(fetcher configstorer.Fetcher) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Create config storer of etcd error. %v", r)
		}
	}()

	err = configstorer.InitStorerWithFetcher(fetcher)

	return
}
