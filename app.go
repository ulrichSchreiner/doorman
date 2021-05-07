package doorman

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/steambap/captcha"
	"go.uber.org/zap"
)

var (
	headersForClient = []string{"X-Real-IP", "X-Forwarded-For"}
	etagHeaders      = []string{
		"ETag",
		"If-Modified-Since",
		"If-Match",
		"If-None-Match",
		"If-Range",
		"If-Unmodified-Since",
	}
)

const (
	numberBytes  = "0123456789"
	uidField     = "uid"
	mailField    = "email"
	tokenField   = "token"
	captchaField = "captcha"
	dmrequest    = "__dm_request__"
)

func randToken(n int) string {
	// this is for testing purposes
	os.Getenv("DUMMYTOKEN")
	if os.Getenv("DUMMYTOKEN") != "" {
		return os.Getenv("DUMMYTOKEN")
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = numberBytes[rand.Intn(len(numberBytes))]
	}
	return string(b)
}

func spacedToken(spacer string, token string) string {
	if len(spacer) < 1 {
		return token
	}
	spacdigit := spacer[0]

	// as long as our token is only of digits we can use chars and not runes here
	chars := []byte(token)

	var res []byte
	for _, c := range chars {
		res = append(res, c, spacdigit)
	}
	return string(res[0 : len(res)-1])
}

type appHandlerFunc func(w http.ResponseWriter, r *http.Request) (result, int)

// A Result simply transports a result and a message string to the client.
type result struct {
	Message  string            `json:"message"`
	Reload   bool              `json:"reload"`
	Register bool              `json:"register"`
	Data     map[string]string `json:"data,omitempty"`
}

type uiconfig struct {
	Imprint       string `json:"imprint"`
	PrivacyPolicy string `json:"privacy_policy"`
	OperationMode string `json:"operation_mode"`
	CaptchaMode   string `json:"captcha_mode"`
	DurationSecs  int    `json:"duration_secs"`
}

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func (m *MiddlewareApp) IsAppRequest(r *http.Request) bool {
	if r.Header.Get("Content-Type") == "multipart/form-data" {
		_ = r.ParseMultipartForm(1024 * 1024)
	} else {
		_ = r.ParseForm()
	}
	if m.authHost == r.Host {
		return r.Form.Get(dmrequest) != ""
	}
	return false
}

func (m *MiddlewareApp) ServeApp(w http.ResponseWriter, r *http.Request, clip string) {
	pt := r.URL.Path
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		w = gzipResponseWriter{Writer: gz, ResponseWriter: w}
	}

	m.logger.Info("new request", zap.String("path", pt), zap.String("clientip", clip), zap.String("method", r.Method))

	switch pt {
	case "/sendUser":
		appFunc(m.logger, w, r, m.sendUser)
		return
	case "/createCaptcha":
		appFunc(m.logger, w, r, m.createCaptcha)
		return
	case "/waitFor":
		appFunc(m.logger, w, r, m.waitFor)
		return
	case "/allow":
		m.allowSignin(w, r)
		return
	case "/checkToken":
		appFunc(m.logger, w, r, m.checkToken)
		return
	case "/checkOTP":
		appFunc(m.logger, w, r, m.checkOTP)
		return
	case "/register":
		appFunc(m.logger, w, r, m.register)
		return
	case "/fetchTempRegister":
		appFunc(m.logger, w, r, m.fetchTempRegister)
		return
	case "/validateTempRegister":
		appFunc(m.logger, w, r, m.validateTempRegister)
		return
	case "/uisettings":
		m.uisettings(w, r)
		return
	}
	_, err := m.assetsDir.Open(r.URL.Path)
	if err != nil {
		r.URL.Path = "/"
	}

	for _, v := range etagHeaders {
		if r.Header.Get(v) != "" {
			r.Header.Del(v)
		}
	}
	w.Header().Add("Pragma", "no-cache")
	w.Header().Add("Cache-Control", "no-cache")
	w.Header().Add("Expires", "0")

	m.assets.ServeHTTP(w, r)
}

func (m *MiddlewareApp) uisettings(w http.ResponseWriter, r *http.Request) {
	ucfg := uiconfig{
		Imprint:       m.ImprintURL,
		PrivacyPolicy: m.PrivacyPolicyURL,
		OperationMode: string(m.OperationMode),
		CaptchaMode:   string(m.CaptchaMode),
		DurationSecs:  int(time.Duration(m.TokenDuration) / time.Second),
	}
	w.Header().Add("content-type", "application/json")
	if err := json.NewEncoder(w).Encode(ucfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (m *MiddlewareApp) sendToken(ue *UserEntry, w http.ResponseWriter, r *http.Request) (int64, string, int) {
	msg := ""
	rc := http.StatusOK
	created := m.clock.Now().UTC().Unix()
	createdTS, err := m.store.tokensrv.checkTempToken(m.logger, m.Issuer, ue.UID)
	if err != nil {
		// only create/send a token when we dont have one pending
		token := randToken(6)
		m.secCookie.set(w, cookieData{
			uidField:   ue.UID,
			mailField:  ue.EMail,
			tokenField: token,
		})
		// DANGER: logging the token should only be done in DEBUG mode!
		m.logger.Debug("send token", zap.String("token", token), zap.String(uidField, ue.UID))
		if err := m.sendMessage(ue, "Your login token", spacedToken(m.Spacing, token), "Your token: "+token); err != nil {
			msg = fmt.Sprintf("Cannot send message: %s", err.Error())
			rc = http.StatusInternalServerError
		}
		_ = m.store.tokensrv.newTempToken(m.logger, m.Issuer, ue.UID, fmt.Sprintf("%d", created), time.Duration(m.TokenDuration))
	} else {
		_, _ = fmt.Sscanf(createdTS, "%d", &created)
		m.logger.Info("token already sent", zap.String("uid", ue.UID))
	}
	return created, msg, rc
}

func (m *MiddlewareApp) sendYesNoLink(ue *UserEntry, w http.ResponseWriter, r *http.Request) (string, string, int) {
	msg := ""
	rc := http.StatusOK
	key := randomKey(8)
	m.secCookie.set(w, cookieData{
		uidField:  ue.UID,
		mailField: ue.EMail,
	})
	// DANGER: logging the token should only be done in DEBUG mode!
	m.logger.Debug("send yesno link", zap.String("key", key), zap.String(uidField, ue.UID))
	link := fmt.Sprintf("%s/allow?t=%s&%s=1", m.IssuerBase, url.QueryEscape(key), dmrequest)

	var buf bytes.Buffer
	if err := emailLinkNotification.Execute(&buf, map[string]string{
		"Loginlink":     link,
		"Timeout":       time.Duration(m.TokenDuration).String(),
		"Sent":          time.Now().UTC().Format(time.RFC3339),
		"Imprint":       m.ImprintURL,
		"PrivacyPolicy": m.PrivacyPolicyURL,
	}); err != nil {
		msg = fmt.Sprintf("Cannot render mail template: %s", err.Error())
		rc = http.StatusInternalServerError
	} else {
		if err := m.sendMessage(ue, "Your login request", "Signin: "+link, buf.String()); err != nil {
			msg = fmt.Sprintf("Cannot send message: %s", err.Error())
			rc = http.StatusInternalServerError
		}
	}
	return key, msg, rc
}

func (m *MiddlewareApp) validateTempRegister(w http.ResponseWriter, r *http.Request) (rs result, rc int) {
	if err := r.ParseMultipartForm(1024); err != nil {
		m.logger.Error("cannot parse form", zap.Error(err))
		rc = http.StatusInternalServerError
		return
	}
	uid := r.FormValue("uid")
	key := r.FormValue("key")
	token := r.FormValue("token")
	if err := m.store.tokensrv.validateTempRegistration(m.logger, m.Issuer, uid, key, token); err != nil {
		rs.Message = "Cannot validate registration"
		rc = http.StatusForbidden
	}
	return
}

func (m *MiddlewareApp) fetchTempRegister(w http.ResponseWriter, r *http.Request) (rs result, rc int) {
	if err := r.ParseMultipartForm(1024); err != nil {
		m.logger.Error("cannot parse form", zap.Error(err))
		rc = http.StatusInternalServerError
		return
	}
	uid := r.FormValue("uid")
	qrimage, err := m.store.tokensrv.qrImage(m.logger, uid, r.FormValue("key"))
	if err != nil {
		m.logger.Error("cannot create QR image", zap.Error(err))
		rs.Message = "Cannot create QR image"
		rc = http.StatusInternalServerError
		return
	}

	rs.Data = map[string]string{
		"image": qrimage,
	}
	return
}

func (m *MiddlewareApp) register(w http.ResponseWriter, r *http.Request) (rs result, rc int) {
	data, err := m.secCookie.get(r)
	if err != nil {
		m.logger.Error("cannot parse cookie", zap.Error(err))
		rs.Message = "Illegal values in cookie"
		rc = http.StatusInternalServerError
		return

	}
	uid, ok := data[uidField].(string)
	if !ok {
		m.logger.Error("illegal value for uid", zap.Any("uid", data[uidField]))
		rs.Message = "Illegal value for UID"
		rc = http.StatusInternalServerError
		return
	}
	email, ok := data[mailField].(string)
	if !ok {
		m.logger.Error("illegal value for email", zap.Any("email", data[mailField]))
		rs.Message = "Illegal value for EMail"
		rc = http.StatusInternalServerError
		return
	}
	if err := m.checkTempRegistration(m.logger, uid); err != nil {
		// only create a temp registration if there is no existing/pending registration
		var key string
		if key, err = m.createTempRegistration(m.logger, uid); err != nil {
			m.logger.Error("cannot create temp registration", zap.Error(err))
			rs.Message = "Cannot create temporary registration"
			rc = http.StatusInternalServerError
		}
		// an otp registration must be sent via EMail (at the momemt)
		if err = m.sendOTPRegistration(uid, email, "", key); err != nil {
			m.logger.Error("cannot send registration", zap.Error(err))
			rs.Message = "Cannot send registration link"
			rc = http.StatusInternalServerError
		}
	}

	return
}

func (m *MiddlewareApp) allowSignin(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	tok := r.FormValue("t")
	allow := r.FormValue("a")
	if allow == "" {
		u, ip, err := m.store.blockinfo(m.logger, tok)
		if err != nil {
			fmt.Fprintln(w, noSigninRequestHTML)
			return
		}
		fmt.Fprintf(w, signinRequestHTML, u, ip, url.QueryEscape(tok))
		return
	}
	y := yesno(allow)
	if y.Yes() {
		_ = m.store.unblock(m.logger, tok, y, time.Duration(m.TokenDuration))
	}
	fmt.Fprintln(w, signinAcceptedHTML)
}

func (m *MiddlewareApp) waitFor(w http.ResponseWriter, r *http.Request) (rs result, rc int) {
	ip := findClientIP(r)
	if err := r.ParseMultipartForm(1024); err != nil {
		m.logger.Error("cannot parse form", zap.Error(err))
		rc = http.StatusInternalServerError
		return
	}
	key := r.FormValue(tokenField)
	data, err := m.secCookie.get(r)
	if err != nil {
		m.logger.Error("cannot parse cookie", zap.Error(err))
		rc = http.StatusInternalServerError
		rs.Message = "Internal Error"
	}
	uid, ok := data[uidField].(string)
	if !ok {
		m.logger.Error("illegal user in cookie", zap.String("uid", uid))
		rc = http.StatusForbidden
		return
	}
	ynw, err := m.store.block(m.logger, uid, ip, key, time.Duration(m.TokenDuration))
	if err != nil {
		m.logger.Error("cannot create blocker", zap.Error(err))
		rc = http.StatusInternalServerError
		rs.Message = "Cannot create blocker"
		return
	}
	yn, err := ynw.WaitFor()
	if err != nil {
		m.logger.Error("waiting returned error", zap.Error(err))
	} else {
		m.logger.Info("waiting returned answer", zap.String("answer", string(*yn)))
		if yn.Yes() {
			m.allowUserIP(m.Issuer, uid, ip)
		}
	}
	rs.Reload = true
	return
}

func (m *MiddlewareApp) createCaptchaRaw() (string, string, error) {
	var capdata *captcha.Data
	var err error
	switch m.CaptchaMode {
	case captchaFull:
		capdata, err = captcha.New(250, 50)
	case captchaMath:
		capdata, err = captcha.NewMathExpr(250, 50)
	case captchaNone:
		return "", "", fmt.Errorf("no captcha mode configured")

	}
	if err != nil {
		return "", "", err
	}

	var buf bytes.Buffer
	if err := capdata.WriteImage(&buf); err != nil {
		return "", "", fmt.Errorf("cannot write captcha to buffer: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), capdata.Text, nil
}

func (m *MiddlewareApp) createCaptcha(w http.ResponseWriter, r *http.Request) (rs result, rc int) {
	capval, captext, err := m.createCaptchaRaw()

	if err != nil {
		m.logger.Error("captcha creation error", zap.Error(err))
		rs.Message = "Captcha error"
		rc = http.StatusInternalServerError
		return
	}
	m.secCookie.set(w, cookieData{
		captchaField: captext,
	})
	rs.Data = map[string]string{
		captchaField: capval,
	}
	return
}

func (m *MiddlewareApp) sendUser(w http.ResponseWriter, r *http.Request) (rs result, rc int) {
	if err := r.ParseMultipartForm(1024); err != nil {
		m.logger.Error("cannot parse form", zap.Error(err))
		rc = http.StatusInternalServerError
		return
	}

	uid := r.FormValue(uidField)
	cap := r.FormValue(captchaField)
	if m.CaptchaMode != captchaNone {
		data, err := m.secCookie.get(r)
		if err != nil {
			m.logger.Debug("no values found in cookie", zap.Error(err))
			rs.Message = "No values found"
			rc = http.StatusInternalServerError
			return
		}
		capdata, ok := data[captchaField]
		if !ok {
			rs.Message = "No captcha found"
			rc = http.StatusForbidden
			return
		}
		if cap != capdata {
			rs.Message = "Wrong captcha data"
			rc = http.StatusForbidden
			capval, captext, _ := m.createCaptchaRaw()
			m.secCookie.set(w, cookieData{
				captchaField: captext,
			})
			rs.Data = map[string]string{
				captchaField: capval,
			}
			return
		}
	}
	if uid != "" {
		ue, err := m.searchUser(uid)
		if err != nil {
			m.logger.Error("cannot find user", zap.String("uid", uid), zap.Error(err))
			rs.Message = "Unknown user: " + uid
			rc = http.StatusForbidden
			if m.CaptchaMode != captchaNone {
				capval, captext, _ := m.createCaptchaRaw()
				m.secCookie.set(w, cookieData{
					captchaField: captext,
				})
				rs.Data = map[string]string{
					captchaField: capval,
				}
			}
			return
		} else {
			m.secCookie.set(w, cookieData{
				uidField:  ue.UID,
				mailField: ue.EMail,
			})
			clip := findClientIP(r)
			if ipallowed := m.store.isIPAllowed(m.logger, clip); ipallowed {
				rs.Reload = true
				rs.Message = "please reload"
				return
			}

			if m.OperationMode.isToken() {
				c, m, rtc := m.sendToken(ue, w, r)
				rs.Data = map[string]string{
					"created": fmt.Sprintf("%d", c),
				}
				rs.Message, rc = m, rtc
				return
			}
			if m.OperationMode.isOTP() {
				has, err := m.store.tokensrv.hasUser(m.logger, m.Issuer, ue.UID)
				if err != nil {
					rs.Message = "Unknown error"
					rc = http.StatusInternalServerError
					return
				}
				rs.Register = !has
				return
			}
			if m.OperationMode.isLink() {
				var key string
				key, rs.Message, rc = m.sendYesNoLink(ue, w, r)
				rs.Data = map[string]string{"key": key}
				return
			}
		}
	} else {
		rs.Message = "Empty userid not allowed"
		rc = http.StatusForbidden
	}
	return
}

func (m *MiddlewareApp) checkOTP(w http.ResponseWriter, r *http.Request) (rs result, rc int) {
	if err := r.ParseMultipartForm(1024); err != nil {
		m.logger.Error("cannot parse form", zap.Error(err))
		rc = http.StatusInternalServerError
		return
	}
	data, err := m.secCookie.get(r)
	formtoken := r.FormValue(tokenField)
	if err == nil {
		uid, ok := data[uidField]
		if !ok {
			rs.Message = "No user found"
			rc = http.StatusForbidden
			return
		}
		ok, err := m.store.tokensrv.validateUser(m.logger, m.Issuer, uid.(string), formtoken)
		if err != nil {
			m.logger.Error("cannot validate user", zap.Error(err))
			rs.Message = "Cannot validate"
			rc = http.StatusInternalServerError
			return
		}
		if !ok {
			rs.Message = "Wrong OTP given"
			rc = http.StatusForbidden
			return
		}
		m.allowUserIP(m.Issuer, uid.(string), findClientIP(r))
	} else {
		m.logger.Debug("no values found in cookie", zap.Error(err))
		rs.Message = "No values found"
	}
	return
}

func (m *MiddlewareApp) checkToken(w http.ResponseWriter, r *http.Request) (rs result, rc int) {
	if err := r.ParseMultipartForm(1024); err != nil {
		m.logger.Error("cannot parse form", zap.Error(err))
		rc = http.StatusInternalServerError
		return
	}
	data, err := m.secCookie.get(r)
	formtoken := r.FormValue(tokenField)
	if err == nil {
		m.logger.Debug("compare cookie and form")
		uid, ok := data[uidField]
		if !ok {
			rs.Message = "No user found"
			rc = http.StatusForbidden
			return
		}
		token, ok := data[tokenField]
		if !ok {
			rs.Message = "No stored token found"
			rc = http.StatusForbidden
			return
		}
		if token.(string) != formtoken {
			m.logger.Debug("given token is invalid", zap.String("formtoken", formtoken))
			rs.Message = "Invalid token"
			rc = http.StatusForbidden
			return
		}
		m.secCookie.set(w, cookieData{
			uidField: uid,
		})
		m.logger.Info("allow user", zap.String(uidField, uid.(string)))
		m.allowUserIP(m.Issuer, uid.(string), findClientIP(r))
	} else {
		m.logger.Debug("no values found in cookie", zap.Error(err))
		rs.Message = "No values found"
	}
	return
}

func findClientIP(r *http.Request) string {
	for _, h := range headersForClient {
		v := r.Header.Get(h)
		if v != "" {
			return v
		}
	}
	h, _, _ := net.SplitHostPort(r.RemoteAddr)
	return h
}

func appFunc(l *zap.Logger, w http.ResponseWriter, r *http.Request, af appHandlerFunc) {
	rs, rc := af(w, r)
	if rc == 0 {
		rc = http.StatusOK
	}
	if rc == http.StatusTemporaryRedirect {
		return // do mothing when we redirect
	}
	l.Debug("write result to client", zap.Any("result", rs), zap.Int("status", rc))
	w.Header().Add("content-type", "application/json")
	w.WriteHeader(rc)
	if err := json.NewEncoder(w).Encode(rs); err != nil {
		l.Error("cannot write result to stream", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
