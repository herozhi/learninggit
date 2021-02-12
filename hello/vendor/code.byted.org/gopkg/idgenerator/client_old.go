package idgenerator

import (
	"code.byted.org/gopkg/logs"
)

// InitIdGeneratorClient deprecated, reference README of idgenerator
func InitIdGeneratorClient(namespace string, countspace string) *IdGeneratorClient {
	logs.Error("[idgenerator] InitIdGeneratorClient is deprecated, reference README of idgenerator")
	version = "1.0.0.2"
	return newIdGeneratorClient(namespace, countspace, defaultNeed64Bit)
}

// InitIdGeneratorWrapper deprecated, reference README of idgenerator
func InitIdGeneratorWrapper(namespace string, countspace string, bit64 bool, count int) *IdGeneratorWrapper {
	logs.Error("[idgenerator] InitIdGeneratorWrapper is deprecated, reference README of idgenerator")
	version = "1.0.0.2"
	client := InitIdGeneratorClient(namespace, countspace)
	if bit64 {
		client.Set64Bits()
	}
	wrapper := newIdGeneratorWrapper(client, count)
	return wrapper
}
