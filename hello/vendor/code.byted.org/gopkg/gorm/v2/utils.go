package gorm

import (
	"bytes"
	"context"
	"regexp"
	"strings"
)

func getPostfixedTableName(tableName string) string {
	if strings.HasSuffix(tableName, StressTestTablePostfix) {
		return tableName
	}
	return tableName + StressTestTablePostfix
}

func isTestRequest(ctx context.Context) bool {
	if stressTag, ok := ctx.Value(ContextStressKey).(string); ok {
		return stressTag != ""
	}
	return false
}

func shouldSkipStressTestsForRead(ctx context.Context) bool {
	if skip, ok := ctx.Value(ContextSkipStressForRead).(bool); ok {
		return skip
	}
	return false
}

var (
	tableAsRegexp   = regexp.MustCompile("(?i)((?: |^)`?)([a-zA-Z0-9_]+)(`?\\s+AS )")
	fromTableRegexp = regexp.MustCompile("(?i)(FROM `?)([a-zA-Z0-9_]+)(`?( |\\)))")
)

func replaceWithShadowTable(name string) string {
	nameBytes := []byte(name)

	for idx, re := range []*regexp.Regexp{tableAsRegexp, fromTableRegexp} {
		matches := re.FindAllSubmatchIndex(nameBytes, -1)
		if len(matches) > 0 {
			newName := make([]byte, 0, len(nameBytes))
			newName = append(newName, nameBytes[:matches[0][0]]...)

			for i, match := range matches {
				newName = append(newName, nameBytes[match[0]:match[3]]...)
				postfixedName, changed := getPostfixedTableNameFromBytes(nameBytes[match[4]:match[5]])
				newName = append(newName, postfixedName...)

				// setup alias table
				if changed && idx >= 1 {
					newName = append(newName, []byte(" AS ")...)
					newName = append(newName, []byte(nameBytes[match[4]:match[5]])...)
				}

				newName = append(newName, nameBytes[match[6]:match[7]]...)

				if i == len(matches)-1 {
					newName = append(newName, nameBytes[match[7]:]...)
				} else {
					newName = append(newName, nameBytes[match[7]:matches[i+1][0]]...)
				}
			}

			nameBytes = newName
			break
		}
	}

	return string(nameBytes)
}

func getPostfixedTableNameFromBytes(tableName []byte) (newName []byte, changed bool) {
	if bytes.HasSuffix(tableName, stressTestTablePostfixBytes) {
		return tableName, false
	}

	newName = make([]byte, len(tableName)+len(stressTestTablePostfixBytes))
	copy(newName, tableName)
	copy(newName[len(tableName):], stressTestTablePostfixBytes)

	return newName, true
}
