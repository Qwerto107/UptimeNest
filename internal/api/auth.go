package api

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const cookieName = "uptimenest_session"
const sessionAge = 7 * 24 * time.Hour

type attempt struct {
	Count int
	Until time.Time
}

func tokenHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
func newToken() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

// allowed 限制密码尝试次数，并及时移除过期记录，防止内存无限增长。
func (a *Server) allowed(r *http.Request) (bool, error) {
	settings, err := a.store.Settings(r.Context())
	if err != nil {
		return false, err
	}
	trusted, err := trustedProxyPrefixes(settings.TrustedProxies)
	if err != nil {
		return false, err
	}
	ip := clientIP(r, trusted)
	a.limitMu.Lock()
	defer a.limitMu.Unlock()
	now := time.Now()
	for key, v := range a.attempts {
		if now.After(v.Until) {
			delete(a.attempts, key)
		}
	}
	v := a.attempts[ip]
	if v.Count >= 10 || (v.Count == 0 && len(a.attempts) >= 10000) {
		return false, nil
	}
	if v.Count == 0 {
		v.Until = now.Add(15 * time.Minute)
	}
	v.Count++
	a.attempts[ip] = v
	return true, nil
}

func (a *Server) authenticated(r *http.Request) (bool, error) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return false, nil
	}
	if len(cookie.Value) != 64 {
		return false, nil
	}
	var expires int64
	err = a.store.ReadDB().QueryRowContext(r.Context(), a.store.Q("SELECT expires FROM sessions WHERE token=?"), tokenHash(cookie.Value)).Scan(&expires)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return expires > time.Now().UnixMilli(), nil
}

func (a *Server) auth(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ok, err := a.authenticated(r)
		if err != nil {
			fail(w, err)
			return
		}
		if !ok {
			writeError(w, 401, "请先登录管理员账户")
			return
		}
		next(w, r)
	})
}

func (a *Server) session(w http.ResponseWriter, r *http.Request) {
	var count int
	if err := a.store.ReadDB().QueryRowContext(r.Context(), "SELECT COUNT(*) FROM account").Scan(&count); err != nil {
		fail(w, err)
		return
	}
	ok, err := a.authenticated(r)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"initialized": count > 0, "authenticated": ok})
}

func (a *Server) setup(w http.ResponseWriter, r *http.Request) {
	a.setupMu.Lock()
	defer a.setupMu.Unlock()
	// 初始化完成后不再消耗登录额度，也不重复计算密码哈希。
	var count int
	if err := a.store.ReadDB().QueryRowContext(r.Context(), "SELECT COUNT(*) FROM account").Scan(&count); err != nil {
		fail(w, err)
		return
	}
	if count > 0 {
		writeError(w, 409, "管理员已设置，请直接登录")
		return
	}
	allowed, err := a.allowed(r)
	if err != nil {
		fail(w, err)
		return
	}
	if !allowed {
		writeError(w, 429, "尝试次数过多，请 15 分钟后重试")
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if !decode(w, r, &body) {
		return
	}
	if !validPassword(w, body.Password) {
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), 12)
	if err != nil {
		fail(w, err)
		return
	}
	// 唯一主键保证首次设置在多进程场景下也只能成功一次。
	result, err := a.store.DB.ExecContext(r.Context(), a.store.Q("INSERT INTO account(id,password_hash) VALUES(1,?) ON CONFLICT(id) DO NOTHING"), string(hash))
	if err != nil {
		fail(w, err)
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		writeError(w, 409, "管理员已设置，请直接登录")
		return
	}
	if err = a.issueSession(w, r); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 201, map[string]bool{"ok": true})
}

func validPassword(w http.ResponseWriter, password string) bool {
	if len(password) == 0 || len(password) > 72 {
		writeError(w, 400, "密码不能为空，且 UTF-8 编码后不能超过 72 字节")
		return false
	}
	return true
}

func (a *Server) login(w http.ResponseWriter, r *http.Request) {
	allowed, err := a.allowed(r)
	if err != nil {
		fail(w, err)
		return
	}
	if !allowed {
		writeError(w, 429, "尝试次数过多，请 15 分钟后重试")
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if !decode(w, r, &body) {
		return
	}
	if !validPassword(w, body.Password) {
		return
	}
	var hash string
	err = a.store.ReadDB().QueryRowContext(r.Context(), "SELECT password_hash FROM account WHERE id=1").Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, 409, "请先设置管理员密码")
		return
	}
	if err != nil {
		fail(w, err)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)) != nil {
		writeError(w, 401, "密码不正确")
		return
	}
	if err = a.issueSession(w, r); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *Server) setCookie(w http.ResponseWriter, token string, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: token, Path: "/api/admin", MaxAge: maxAge, HttpOnly: true, Secure: a.config.CookieSecure, SameSite: http.SameSiteStrictMode})
}

func (a *Server) issueSession(w http.ResponseWriter, r *http.Request) error {
	token, err := newToken()
	if err != nil {
		return err
	}
	_, err = a.store.DB.ExecContext(r.Context(), a.store.Q("INSERT INTO sessions(token,expires) VALUES(?,?)"), tokenHash(token), time.Now().Add(sessionAge).UnixMilli())
	if err == nil {
		a.setCookie(w, token, int(sessionAge.Seconds()))
	}
	return err
}

func (a *Server) logout(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(cookieName)
	if cookie != nil {
		if _, err := a.store.DB.ExecContext(r.Context(), a.store.Q("DELETE FROM sessions WHERE token=?"), tokenHash(cookie.Value)); err != nil {
			fail(w, err)
			return
		}
	}
	a.setCookie(w, "", -1)
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *Server) password(w http.ResponseWriter, r *http.Request) {
	allowed, err := a.allowed(r)
	if err != nil {
		fail(w, err)
		return
	}
	if !allowed {
		writeError(w, 429, "尝试次数过多，请 15 分钟后重试")
		return
	}
	var body struct {
		Current  string `json:"current"`
		Password string `json:"password"`
	}
	if !decode(w, r, &body) {
		return
	}
	if !validPassword(w, body.Password) {
		return
	}
	var hash string
	if err := a.store.ReadDB().QueryRowContext(r.Context(), "SELECT password_hash FROM account WHERE id=1").Scan(&hash); err != nil {
		fail(w, err)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Current)) != nil {
		writeError(w, 400, "当前密码不正确")
		return
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(body.Password), 12)
	if err != nil {
		fail(w, err)
		return
	}
	token, err := newToken()
	if err != nil {
		fail(w, err)
		return
	}
	tx, err := a.store.DB.BeginTx(r.Context(), nil)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(r.Context(), a.store.Q("UPDATE account SET password_hash=? WHERE id=1 AND password_hash=?"), string(newHash), hash)
	if err != nil {
		fail(w, err)
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		writeError(w, 409, "密码已更新，请重新登录")
		return
	}
	if _, err = tx.ExecContext(r.Context(), "DELETE FROM sessions"); err != nil {
		fail(w, err)
		return
	}
	if _, err = tx.ExecContext(r.Context(), a.store.Q("INSERT INTO sessions(token,expires) VALUES(?,?)"), tokenHash(token), time.Now().Add(sessionAge).UnixMilli()); err != nil {
		fail(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		fail(w, err)
		return
	}
	a.setCookie(w, token, int(sessionAge.Seconds()))
	writeJSON(w, 200, map[string]bool{"ok": true})
}
