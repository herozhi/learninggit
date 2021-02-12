package gorm

import (
	"strings"

	"gorm.io/gorm"
)

func StressTestCallback(readMode bool) func(db *gorm.DB) {
	return func(db *gorm.DB) {
		if !db.DryRun && isTestRequest(db.Statement.Context) {
			if readMode && shouldSkipStressTestsForRead(db.Statement.Context) {
				// Skip shadow table when ContextSkipStressForRead is true
				return
			}

			// Use shadow table
			if db.Statement.SQL.String() == "" {
				if db.Statement.TableExpr != nil {
					// Irregular Table, for example:
					//   DB.Table("users as u").Find(&users)
					//   DB.Table("`users` as u").Find(&users)
					//   DB.Table("(?) as u", db.Model(User{}).Select("Name")).Find(&users)
					db.Statement.TableExpr.SQL = replaceWithShadowTable(db.Statement.TableExpr.SQL)
				} else {
					db.Statement.Table = getPostfixedTableName(db.Statement.Table)
				}
			} else {
				// Raw SQL, for example:
				//   DB.Raw("select sum(age) from users where name = ?", "name").Scan(&ages)
				// Skip shadow table when SELECT SQL & ContextSkipStressForRead is true
				rawSQL := db.Statement.SQL.String()
				if shouldSkipStressTestsForRead(db.Statement.Context) && len(rawSQL) > 6 && strings.EqualFold(rawSQL[:6], "select") {
					return
				}

				db.Statement.SQL.Reset()
				db.Statement.SQL.WriteString(replaceWithShadowTable(rawSQL))
			}
		}
	}
}
