package store

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const encryptedPrefix = "enc:v1:"

func defaultKeyFile(dialect, dsn string) string {
	if dialect != "sqlite" {
		return "data/secret.key"
	}
	if sqliteMemory(dsn) {
		return ""
	}
	name, _, _ := strings.Cut(dsn, "?")
	name = strings.TrimPrefix(name, "file:")
	if decoded, err := url.PathUnescape(name); err == nil {
		name = decoded
	}
	return name + ".key"
}

func loadSecretKey(path string, create bool) ([]byte, error) {
	if path == "" {
		if !create {
			return nil, errors.New("加密数据库需要持久化密钥文件")
		}
		key := make([]byte, 32)
		_, err := rand.Read(key)
		return key, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) && create {
		if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return nil, err
		}
		key := make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			return nil, err
		}
		// 排他创建并同步到磁盘；写入失败时保留文件，禁止自动替换已有密钥。
		file, openErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if errors.Is(openErr, os.ErrExist) {
			return loadSecretKey(path, false)
		}
		if openErr != nil {
			return nil, openErr
		}
		_, err = file.WriteString(base64.StdEncoding.EncodeToString(key) + "\n")
		if err == nil {
			err = file.Sync()
		}
		err = errors.Join(err, file.Close())
		if err == nil && runtime.GOOS != "windows" {
			// 同步目录项，避免首次启动断电后数据库已提交而新密钥文件丢失。
			directory, openErr := os.Open(filepath.Dir(path))
			if openErr != nil {
				return nil, openErr
			}
			err = errors.Join(directory.Sync(), directory.Close())
		}
		return key, err
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("加密密钥文件缺失，请从备份恢复原密钥")
	}
	if err != nil {
		return nil, err
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil || len(key) != 32 {
		return nil, errors.New("加密密钥文件格式无效，需要 Base64 编码的 32 字节密钥")
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if info.Mode().Perm()&0077 != 0 {
			if err = os.Chmod(path, 0600); err != nil {
				return nil, err
			}
		}
	}
	return key, nil
}

func secretDomain(table string, id int64) string {
	return table + "/" + strconv.FormatInt(id, 10)
}

func (s *Store) sealJSON(domain string, value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	// 标准库生成并附加随机 nonce，附加认证数据绑定表和记录身份。
	sealed := s.secrets.Seal(nil, nil, data, []byte(domain))
	return encryptedPrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

func (s *Store) openJSON(domain, data string, value any) error {
	if !strings.HasPrefix(data, encryptedPrefix) {
		return errors.New("敏感配置尚未加密，请停止服务并重新启动以完成迁移")
	}
	sealed, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(data, encryptedPrefix))
	if err != nil {
		return errors.New("加密配置格式无效")
	}
	plain, err := s.secrets.Open(nil, nil, sealed, []byte(domain))
	if err != nil {
		return errors.New("无法解密配置，请检查密钥文件是否匹配或数据是否损坏")
	}
	return json.Unmarshal(plain, value)
}

func (s *Store) initEncryption(ctx context.Context, path string) error {
	var check string
	err := s.DB.QueryRowContext(ctx, "SELECT value FROM settings WHERE key='encryption_check'").Scan(&check)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	key, err := loadSecretKey(path, check == "")
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	s.secrets, err = cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return err
	}
	if check != "" {
		var value string
		if err = s.openJSON("settings/encryption_check", check, &value); err != nil {
			return err
		}
		if value != "uptimenest-secrets-v1" {
			return errors.New("加密密钥校验失败")
		}
		if _, err = s.DB.ExecContext(ctx, "INSERT INTO schema_version(version) VALUES(5) ON CONFLICT(version) DO NOTHING"); err != nil {
			return err
		}
		return s.encryptDeliveries(ctx)
	}
	if s.dialect == "sqlite" {
		// 迁移前清零被替换的单元内容，提交后截断旧 WAL，减少明文残留。
		if _, err = s.DB.ExecContext(ctx, "PRAGMA secure_delete=ON"); err != nil {
			return err
		}
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, table := range []string{"monitors", "notification_channels", "alert_states", "notification_deliveries"} {
		if err = s.encryptLegacyRows(ctx, tx, table); err != nil {
			return err
		}
	}
	check, err = s.sealJSON("settings/encryption_check", "uptimenest-secrets-v1")
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, s.Q("INSERT INTO settings(key,value) VALUES('encryption_check',?)"), check); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO schema_version(version) VALUES(5),(6) ON CONFLICT(version) DO NOTHING"); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return s.checkpointEncryption(ctx)
}

// encryptDeliveries 为已升级到版本 5 的库补充加密通知事件；已加密的行只校验不重写。
func (s *Store) encryptDeliveries(ctx context.Context) error {
	var version int
	if err := s.DB.QueryRowContext(ctx, "SELECT COALESCE(MAX(version),0) FROM schema_version").Scan(&version); err != nil {
		return err
	}
	if version >= 6 {
		return s.checkpointEncryption(ctx)
	}
	if s.dialect == "sqlite" {
		if _, err := s.DB.ExecContext(ctx, "PRAGMA secure_delete=ON"); err != nil {
			return err
		}
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = s.encryptLegacyRows(ctx, tx, "notification_deliveries"); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO schema_version(version) VALUES(6) ON CONFLICT(version) DO NOTHING"); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return s.checkpointEncryption(ctx)
}

func (s *Store) checkpointEncryption(ctx context.Context) error {
	if s.dialect == "sqlite" {
		var busy, frames, checkpointed int
		if err := s.DB.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &frames, &checkpointed); err != nil {
			return err
		}
		if busy != 0 {
			return errors.New("加密迁移已提交，但 WAL 仍被占用，请关闭其他数据库连接后重启")
		}
	}
	return nil
}

func (s *Store) encryptLegacyRows(ctx context.Context, tx *sql.Tx, table string) error {
	column := "data"
	if table == "notification_deliveries" {
		column = "event"
	}
	query := "SELECT id,'' AS kind," + column + " FROM " + table
	if table == "alert_states" {
		query = "SELECT monitor_id,kind,data FROM alert_states"
	}
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	type row struct {
		id   int64
		kind string
		data string
	}
	items := []row{}
	for rows.Next() {
		var item row
		if err = rows.Scan(&item.id, &item.kind, &item.data); err != nil {
			rows.Close()
			return err
		}
		items = append(items, item)
	}
	err = errors.Join(rows.Err(), rows.Close())
	if err != nil {
		return err
	}
	for _, item := range items {
		domain := secretDomain(table, item.id)
		if table == "alert_states" {
			domain += "/" + item.kind
		}
		if strings.HasPrefix(item.data, encryptedPrefix) {
			var value json.RawMessage
			if err = s.openJSON(domain, item.data, &value); err != nil {
				return err
			}
			continue
		}
		sealed, err := s.sealJSON(domain, json.RawMessage(item.data))
		if err != nil {
			return fmt.Errorf("迁移 %s 加密配置失败", table)
		}
		query := "UPDATE " + table + " SET " + column + "=? WHERE id=?"
		args := []any{sealed, item.id}
		if table == "alert_states" {
			query = "UPDATE alert_states SET data=? WHERE monitor_id=? AND kind=?"
			args = append(args, item.kind)
		}
		if _, err = tx.ExecContext(ctx, s.Q(query), args...); err != nil {
			return err
		}
	}
	return nil
}
