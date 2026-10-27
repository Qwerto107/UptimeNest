package store

import (
	"net/url"
	"strings"
)

func sqliteMemory(dsn string) bool {
	name, query, _ := strings.Cut(dsn, "?")
	values, _ := url.ParseQuery(query)
	return name == ":memory:" || name == "file::memory:" || values.Get("mode") == "memory"
}

// sqliteDSN 在读写池的每个新建或重建连接上应用外键、繁忙等待和 WAL。
// 读池禁止写入，写事务提前获得写锁，避免只在迁移连接上设置 PRAGMA。
func sqliteDSN(dsn string, readOnly bool) (string, error) {
	name, query, _ := strings.Cut(dsn, "?")
	values, err := url.ParseQuery(query)
	if err != nil {
		return "", err
	}
	// 同步覆盖驱动别名，避免自定义 DSN 中的旧参数抵消应用约束。
	values.Set("_foreign_keys", "on")
	values.Set("_fk", "on")
	values.Set("_busy_timeout", "5000")
	values.Set("_timeout", "5000")
	values.Set("_journal_mode", "WAL")
	values.Set("_journal", "WAL")
	if readOnly {
		values.Set("_txlock", "deferred")
		values.Set("_query_only", "on")
	} else {
		values.Set("_txlock", "immediate")
	}
	return name + "?" + values.Encode(), nil
}
