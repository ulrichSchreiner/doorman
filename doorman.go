package doorman

import (
	"bytes"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"text/template"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

type operationMode string

const (
	defaultTokenDuration  = Duration(1 * time.Minute)
	defaultAccessDuration = Duration(10 * time.Hour)
	operationsModeToken   = operationMode("token")
	operationsModeOTP     = operationMode("otp")
	operationsModeLink    = operationMode("link")
)

const accessAllowed = `
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
		<h3>Access allowed</h3>
</body>
</html>
`

//go:embed webapp/dist/*
var embeddedAsset embed.FS

var (
	validOpModes = map[operationMode]bool{
		operationsModeOTP:   true,
		operationsModeToken: true,
		operationsModeLink:  true,
	}
)

func (op *operationMode) isToken() bool {
	return *op == operationsModeToken
}

func (op *operationMode) isOTP() bool {
	return *op == operationsModeOTP
}

func (op *operationMode) isLink() bool {
	return *op == operationsModeLink
}

func init() {
	caddy.RegisterModule(MiddlewareApp{})
	caddy.RegisterModule(Middleware{})
}

type Duration time.Duration

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		*d = Duration(time.Duration(value))
		return nil
	case string:
		tmp, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		*d = Duration(tmp)
		return nil
	default:
		return errors.New("invalid duration")
	}
}

type TypedPlugin struct {
	Type string          `json:"type"`
	Name string          `json:"name"`
	Spec json.RawMessage `json:"spec"`
}

type Plugins []TypedPlugin

type RedisConfig struct {
	Address  string `json:"address"`
	Password string `json:"password,omitempty"`
	DB       int    `json:"db,omitempty"`
	client   *redis.Client
}

type MessengerConfig struct {
	Burst int      `json:"burst"`
	Rate  Duration `json:"rate"`
	From  struct {
		Name  string `json:"name"`
		EMail string `json:"email"`
	} `json:"from"`
	Transports Plugins `json:"transports"`
}

type StoreSettings struct {
	PersistentType string       `json:"persistent_type"`
	MemCacheMB     int          `json:"memory_cache_mb"`
	Redis          *RedisConfig `json:"redis,omitempty"`
	OTP            struct {
		Timeout          Duration     `json:"timeout,omitempty"`
		RegisterTemplate string       `json:"register_template,omitempty"`
		Transport        *TypedPlugin `json:"transport,omitempty"`
		registerTemplate *template.Template
		transport        messageTransport
	} `json:"otp"`
}

// MiddlewareApp implements an HTTP handler
type MiddlewareApp struct {
	Users            Plugins         `json:"users,omitempty"`
	Whitelist        Plugins         `json:"whitelist,omitempty"`
	CookieHash       []byte          `json:"cookie_hash"`
	CookieBlock      []byte          `json:"cookie_block"`
	InsecureCookie   bool            `json:"insecure_cookie,omitempty"`
	Domain           string          `json:"domain,omitempty"`
	Issuer           string          `json:"issuer,omitempty"`
	IssuerBase       string          `json:"issuer_base"`
	Spacing          string          `json:"spacing,omitempty"`
	OperationMode    operationMode   `json:"operation_mode"`
	Channels         []string        `json:"channels"`
	AccessDuration   Duration        `json:"access_duration"`
	TokenDuration    Duration        `json:"token_duration"`
	Messenger        MessengerConfig `json:"messenger_config"`
	StoreSettings    StoreSettings   `json:"store_settings"`
	ImprintURL       string          `json:"imprint_url"`
	PrivacyPolicyURL string          `json:"privacy_policy_url"`
	logger           *zap.Logger
	store            *persistentStore
	secCookie        *cookieHandler
	clock            clock.Clock
	assets           http.Handler
	assetsDir        http.FileSystem
	transporters     transporters
	userbackends     *userBackends
	whitelister      *whitelister
	authHost         string
}

// CaddyModule returns the Caddy module information.
func (MiddlewareApp) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "doorman",
		New: func() caddy.Module { return new(MiddlewareApp) },
	}
}

func (m *MiddlewareApp) Start() error {
	return nil
}

func (m *MiddlewareApp) Stop() error {
	return nil
}

// Provision implements caddy.Provisioner.
func (m *MiddlewareApp) Provision(ctx caddy.Context) error {
	m.logger = ctx.Logger(m)
	m.clock = clock.New()
	var rcli *redis.Client
	if m.StoreSettings.Redis != nil {
		rcli = redis.NewClient(&redis.Options{
			Addr:     m.StoreSettings.Redis.Address,
			Password: m.StoreSettings.Redis.Password,
			DB:       m.StoreSettings.Redis.DB,
		})
		m.StoreSettings.Redis.client = rcli
	}
	if m.StoreSettings.OTP.Transport != nil {
		t, err := fromTransportSpec(m.logger, *m.StoreSettings.OTP.Transport)
		if err != nil {
			return err
		}
		tpl, err := template.New("registerOTP").Parse(m.StoreSettings.OTP.RegisterTemplate)
		if err != nil {
			return fmt.Errorf("cannot parse registerOTP template: %w", err)
		}
		m.StoreSettings.OTP.registerTemplate = tpl
		m.StoreSettings.OTP.transport = t
	}
	if m.StoreSettings.OTP.Timeout == 0 {
		m.StoreSettings.OTP.Timeout = Duration(15 * time.Minute)
	}

	m.secCookie = newCookie(m.logger, m.CookieHash, m.CookieBlock, m.InsecureCookie, m.Domain)

	// we check if there is a local directory named "webapp/dist". when developing the
	// code, we have this directory and so we can easily change the HTML/JS code and test
	// it.
	if _, err := os.Stat("webapp/dist"); os.IsNotExist(err) {
		// no local assets in directory, use embedded ones
		sub, err := fs.Sub(embeddedAsset, "webapp/dist")
		if err == nil {
			m.assetsDir = http.FS(sub)
			goto assetsInitialized
		}
	}
	m.assetsDir = http.Dir("webapp/dist")
assetsInitialized:
	m.assets = http.FileServer(m.assetsDir)
	if m.AccessDuration == 0 {
		m.AccessDuration = defaultAccessDuration
	}
	if m.TokenDuration == 0 {
		m.TokenDuration = defaultTokenDuration
	}
	if m.OperationMode == "" {
		m.OperationMode = operationsModeToken
	}
	trsp, err := fromTransportSpecs(m.logger, m.Messenger.Transports)
	if err != nil {
		return fmt.Errorf("cannot create messenger: %w", err)
	}
	m.transporters = trsp
	ub, err := fromUserSpecs(m.logger, m.Users)
	if err != nil {
		return fmt.Errorf("cannot create user backends: %w", err)
	}
	m.userbackends = ub

	ws, err := fromWhitelistSpecs(m.logger, m.Whitelist)
	if err != nil {
		return fmt.Errorf("cannot create whitelist loaders: %w", err)
	}
	m.whitelister = ws

	store, err := newStore(m.logger, m.clock, m.StoreSettings, ws)
	if err != nil {
		return fmt.Errorf("cannot initialize store: %w", err)
	}
	m.store = store
	m.logger.Info("configdata", zap.Any("config", *m))

	return nil
}

// Validate implements caddy.Validator.
func (m *MiddlewareApp) Validate() error {
	if len(m.CookieBlock) != 32 {
		return fmt.Errorf("the value for cookie_block must be 32 bytes, for example: %v", base64.StdEncoding.EncodeToString(newRandomKey(32)))
	}
	if len(m.CookieHash) != 64 {
		return fmt.Errorf("the value for cookie_hash must be 64 bytes, for example: %v", base64.StdEncoding.EncodeToString(newRandomKey(64)))
	}

	if _, valid := validOpModes[m.OperationMode]; !valid {
		return fmt.Errorf("invalid operation mode: %s", m.OperationMode)
	}
	if m.OperationMode.isOTP() {
		if !m.canDoOTP() {
			return fmt.Errorf("OTP operations configured, but no transport configured")
		}
		if m.Issuer == "" {
			return fmt.Errorf("when using OTP, you must set issuer")
		}
	}
	if m.IssuerBase == "" {
		return fmt.Errorf("you must specify the issuer_base as a base url, aka https://www.example.com")
	}
	if u, e := url.Parse(m.IssuerBase); e != nil {
		return fmt.Errorf("you must specify the issuer_base as a base url, aka https://www.example.com: %w", e)
	} else {
		m.authHost = u.Host
	}
	return nil
}

// UnmarshalCaddyfile implements caddyfile.Unmarshaler.
func (m *MiddlewareApp) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
	}
	return nil
}

func (m *MiddlewareApp) searchUser(uid string) (*UserEntry, error) {
	ue, err := m.userbackends.Search(m.logger, uid)
	if err == nil {
		m.logger.Info("found user", zap.Any("user", ue))
	}
	return ue, err
}

func (m *MiddlewareApp) sendMessage(usr *UserEntry, subject, msg, body string) error {
	m.logger.Info("send message", zap.Any("transporters", m.transporters))
	for _, c := range m.Channels {
		t, ok := m.transporters[c]
		if !ok {
			continue
		}
		a := addressable{
			FromMail: m.Messenger.From.EMail,
			FromName: m.Messenger.From.Name,
			ToMail:   usr.EMail,
			ToMobile: usr.Mobile,
		}
		res, err := t.Send(m.logger, a, subject, msg, body)
		if err != nil {
			m.logger.Error("error with messenger", zap.String("output", res), zap.Error(err))
			return err
		}
		m.logger.Info("message to user sent", zap.String("output", res))
		return nil
	}
	return fmt.Errorf("no message transport found for channels: %v", m.Channels)
}

func (m *MiddlewareApp) createTempRegistration(log *zap.Logger, uid string) (string, error) {
	return m.store.tokensrv.newTempRegistration(log, m.Issuer, uid)
}

func (m *MiddlewareApp) checkTempRegistration(log *zap.Logger, uid string) error {
	return m.store.tokensrv.checkTempRegistration(log, m.Issuer, uid)
}

func (m *MiddlewareApp) sendOTPRegistration(uid, email, mobile, regkey string) error {
	authlink := fmt.Sprintf("%s/#/signup/%s/%s", m.IssuerBase, url.QueryEscape(uid), url.QueryEscape(regkey))
	m.logger.Info("send otp registration", zap.String("email", email), zap.String("link", authlink))
	var body bytes.Buffer
	a := addressable{
		FromMail: m.Messenger.From.EMail,
		FromName: m.Messenger.From.Name,
		ToMail:   email,
		ToMobile: mobile,
	}
	if m.StoreSettings.OTP.registerTemplate != nil {
		data := map[string]string{
			"link":  authlink,
			"uid":   uid,
			"email": email,
		}
		err := m.StoreSettings.OTP.registerTemplate.Execute(&body, data)
		if err != nil {
			return fmt.Errorf("cannot generate email with template: %w", err)
		}
	}
	res, err := m.StoreSettings.OTP.transport.Send(m.logger, a, "Your registration", authlink, body.String())
	if err != nil {
		m.logger.Error("error with otp messenger", zap.String("output", res), zap.Error(err))
		return err
	}
	m.logger.Info("message to user sent", zap.String("output", res))
	return nil
}

func (m *MiddlewareApp) allowUserIP(issuer, userid, clip string) {
	if err := m.store.allowUserIP(m.logger, clip, time.Duration(m.AccessDuration)); err != nil {
		m.logger.Error("cannot allow userip", zap.Error(err))
	}
	m.store.tokensrv.removeTempToken(m.logger, issuer, userid)
}

func (m *MiddlewareApp) canDoOTP() bool {
	return m.StoreSettings.OTP.Transport != nil
}

type Middleware struct {
	app *MiddlewareApp
}

func (Middleware) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.doorman",
		New: func() caddy.Module { return new(Middleware) },
	}
}

// ServeHTTP implements caddyhttp.MiddlewareHandler.
func (m *Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	clip := findClientIP(r)
	ipallowed := m.app.store.isAllowed(clip)
	if r.Host == m.app.authHost {
		if ipallowed {
			w.Header().Add("content-type", "text/html")
			fmt.Fprintln(w, accessAllowed)
			return nil
		}
		m.app.ServeApp(w, r, clip)
		return nil
	}
	m.app.logger.Debug("check if access is allowed", zap.String("clientip", clip), zap.Bool("ipallowed", ipallowed))
	if m.app.store.isAllowed(clip) {
		return next.ServeHTTP(w, r)
	}
	m.app.ServeApp(w, r, clip)
	return nil
}

func (m *Middleware) Provision(ctx caddy.Context) error {
	dm, err := ctx.App("doorman")
	if err != nil {
		return err
	}
	m.app = dm.(*MiddlewareApp)
	return nil
}

// Interface guards
var (
	_ caddy.Provisioner           = (*MiddlewareApp)(nil)
	_ caddy.Validator             = (*MiddlewareApp)(nil)
	_ caddyhttp.MiddlewareHandler = (*Middleware)(nil)
	_ caddyfile.Unmarshaler       = (*MiddlewareApp)(nil)
	_ caddy.App                   = (*MiddlewareApp)(nil)
)
