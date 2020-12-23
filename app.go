package doorman

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"go.uber.org/zap"
)

var (
	headersForClient = []string{"X-Real-IP", "X-Forwarded-For"}
)

const (
	numberBytes = "0123456789"
	uidField    = "uid"
	mailField   = "email"
	tokenField  = "token"
)

const signinRequestHTML = `
<html>
<head>
<meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<style>
* {
  font-family: sans-serif;
  text-align: center;
}
h3 {
	margin-top: 50px;
}
.answer {
  margin-top: 50px;
}
</style>
<body>
		<h3>Signin Request</h3>
		<div>A signin request from user <b>%s</b> originated from IP <b>%s</b></div>
		<div class="answer">Click <a href="allow?t=%s&a=yes">YES</a> to allow this request.
</body>
</html>
`
const noSigninRequestHTML = `
<html>
<head>
<meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<style>
* {
  font-family: sans-serif;
  text-align: center;
}
h3 {
	margin-top: 50px;
}
.answer {
  margin-top: 50px;
}
</style>
<body>
		<h3>Signin Request</h3>
		<div class="answer">No active request found</div>
</body>
</html>
`

const signinAcceptedHTML = `
<html>
<head>
<meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<style>
* {
  font-family: sans-serif;
  text-align: center;
}
h3 {
	margin-top: 50px;
}
</style>
<body>
		<h3>Signin Request accepted</h3>
</body>
</html>
`

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
	DurationSecs  int    `json:"duration_secs"`
}

func (m *Middleware) ServeApp(w http.ResponseWriter, r *http.Request, clip string) {
	pt := r.URL.Path

	m.logger.Info("new request", zap.String("path", pt), zap.String("clientip", clip), zap.String("method", r.Method))

	switch pt {
	case "/sendUser":
		appFunc(m.logger, w, r, m.sendUser)
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

	m.assets.ServeHTTP(w, r)
}

func (m *Middleware) uisettings(w http.ResponseWriter, r *http.Request) {
	ucfg := uiconfig{
		Imprint:       m.ImprintURL,
		PrivacyPolicy: m.PrivacyPolicyURL,
		OperationMode: string(m.OperationMode),
		DurationSecs:  int(time.Duration(m.TokenDuration) / time.Second),
	}
	w.Header().Add("content-type", "application/json")
	if err := json.NewEncoder(w).Encode(ucfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (m *Middleware) sendToken(ue *UserEntry, w http.ResponseWriter, r *http.Request) (int64, string, int) {
	msg := ""
	rc := http.StatusOK
	created := m.clock.Now().UTC().Unix()
	created_ts, err := m.store.tokensrv.checkTempToken(m.logger, m.Issuer, ue.UID)
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
		_, _ = fmt.Sscanf(created_ts, "%d", &created)
		m.logger.Info("token already sent", zap.String("uid", ue.UID))
	}
	return created, msg, rc
}

func (m *Middleware) sendYesNoLink(ue *UserEntry, w http.ResponseWriter, r *http.Request) (string, string, int) {
	msg := ""
	rc := http.StatusOK
	key := randomKey(8)
	m.secCookie.set(w, cookieData{
		uidField:  ue.UID,
		mailField: ue.EMail,
	})
	// DANGER: logging the token should only be done in DEBUG mode!
	m.logger.Debug("send yesno link", zap.String("key", key), zap.String(uidField, ue.UID))
	link := fmt.Sprintf("%s/allow?t=%s", m.IssuerBase, url.QueryEscape(key))

	body := fmt.Sprintf(`A login request was triggered. Click <a href="%s">to allow</a>. After %s the request will be automatically denied`, link, time.Duration(m.TokenDuration).String())

	if err := m.sendMessage(ue, "Your login request", "Signin: "+link, body); err != nil {
		msg = fmt.Sprintf("Cannot send message: %s", err.Error())
		rc = http.StatusInternalServerError
	}
	return key, msg, rc
}

func (m *Middleware) validateTempRegister(w http.ResponseWriter, r *http.Request) (rs result, rc int) {
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

func (m *Middleware) fetchTempRegister(w http.ResponseWriter, r *http.Request) (rs result, rc int) {
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

func (m *Middleware) register(w http.ResponseWriter, r *http.Request) (rs result, rc int) {
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

func (m *Middleware) allowSignin(w http.ResponseWriter, r *http.Request) {
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

func (m *Middleware) waitFor(w http.ResponseWriter, r *http.Request) (rs result, rc int) {
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
			m.allowUserIP(ip)
		}
	}
	rs.Reload = true
	return
}

func (m *Middleware) sendUser(w http.ResponseWriter, r *http.Request) (rs result, rc int) {
	if err := r.ParseMultipartForm(1024); err != nil {
		m.logger.Error("cannot parse form", zap.Error(err))
		rc = http.StatusInternalServerError
		return
	}

	uid := r.FormValue(uidField)
	if uid != "" {
		ue, err := m.searchUser(uid)
		if err != nil {
			m.logger.Error("cannot find user", zap.String("uid", uid), zap.Error(err))
			rs.Message = "Unknown user"
			rc = http.StatusForbidden
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

func (m *Middleware) checkOTP(w http.ResponseWriter, r *http.Request) (rs result, rc int) {
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
		m.allowUserIP(findClientIP(r))
	} else {
		m.logger.Debug("no values found in cookie", zap.Error(err))
		rs.Message = "No values found"
	}
	return
}

func (m *Middleware) checkToken(w http.ResponseWriter, r *http.Request) (rs result, rc int) {
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
		m.allowUserIP(findClientIP(r))
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
