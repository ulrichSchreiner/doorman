package doorman

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"text/template"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/caddyserver/caddy/v2"
	"github.com/go-mail/mail"
	"go.uber.org/zap"
)

type TransportSkill string

type addressable struct {
	FromMail string
	FromName string
	ToMail   string
	ToMobile string
	ToName   string
}

type messageTransport interface {
	//Send(lg *zap.Logger, from, to, tomobile, subject, message, body string) (string, error)
	Send(lg *zap.Logger, a addressable, subject, shortmessage, body string) (string, error)
}

type transporters map[string]messageTransport

var (
	_ messageTransport = (*StdinMsgConfig)(nil)
	_ messageTransport = (*SMTPMsgConfig)(nil)
	_ messageTransport = (*URLMsgConfig)(nil)
	_ messageTransport = (*messageSender)(nil)
)

const (
	valueCommandMessenger = "command"
	valueURLMessenger     = "url"
	valueEMailMessenger   = "email"
)

type StdinMsgConfig struct {
	Command  string   `json:"command"`
	Args     []string `json:"args,omitempty"`
	UseStdin bool     `json:"use_stdin"`
}

func newStdin(cfg StdinMsgConfig, rpl *caddy.Replacer) (*StdinMsgConfig, error) {
	cfg.Command = rpl.ReplaceKnown(cfg.Command, "")
	for i, a := range cfg.Args {
		cfg.Args[i] = rpl.ReplaceKnown(a, "")
	}
	return &cfg, nil
}

func (std *StdinMsgConfig) Send(lg *zap.Logger, a addressable, subject, shortmessage, body string) (string, error) {
	args := append([]string{}, std.Args...)
	if !std.UseStdin {
		args = append(args, shortmessage)
	}

	c := exec.Command(std.Command, args...)
	stdin, err := c.StdinPipe()
	if err != nil {
		return "", err
	}

	go func() {
		defer stdin.Close()
		if std.UseStdin {
			_, _ = io.WriteString(stdin, shortmessage)
		}
	}()

	out, err := c.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

type URLMsgConfig struct {
	URLTemplate  string      `json:"url_template"`
	BodyTemplate string      `json:"body_template"`
	Method       string      `json:"method"`
	Insecure     bool        `json:"insecure"`
	Headers      http.Header `json:"headers,omitempty"`
	AuthUser     string      `json:"auth_user,omitempty"`
	AuthPassword string      `json:"auth_password,omitempty"`
	urlTemplate  *template.Template
	bodyTemplate *template.Template
	client       *http.Client
}

func (um *URLMsgConfig) Send(lg *zap.Logger, a addressable, subject, shortmessage, mbody string) (string, error) {
	data := map[string]string{
		"message":  url.QueryEscape(shortmessage),
		"subject":  url.QueryEscape(subject),
		"tomail":   url.QueryEscape(a.ToMail),
		"toname":   url.QueryEscape(a.ToName),
		"tomobile": url.QueryEscape(a.ToMobile),
		"frommail": url.QueryEscape(a.FromMail),
		"fromname": url.QueryEscape(a.FromName),
		"body":     url.QueryEscape(mbody),
	}
	var buf bytes.Buffer
	err := um.urlTemplate.Execute(&buf, data)
	if err != nil {
		return "", fmt.Errorf("cannot create url: %v", err)
	}
	var body bytes.Buffer
	if um.bodyTemplate != nil {
		err := um.bodyTemplate.Execute(&body, data)
		if err != nil {
			return "", fmt.Errorf("cannot create body: %v", err)
		}
	}
	rq, err := http.NewRequest(um.Method, buf.String(), &body)
	if err != nil {
		return "", fmt.Errorf("cannot create request: %v", err)
	}
	for k, v := range um.Headers {
		rq.Header[k] = v
	}
	if um.AuthUser != "" {
		rq.SetBasicAuth(um.AuthUser, um.AuthPassword)
	}
	lg.Info("invoke messenger", zap.String("url", buf.String()), zap.String("method", um.Method))
	rsp, err := um.client.Do(rq)
	if err != nil {
		return "", fmt.Errorf("cannot do request: %v", err)
	}
	defer rsp.Body.Close()
	content, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return "", fmt.Errorf("cannot read response: %v", err)
	}
	if rsp.StatusCode/100 != 2 {
		return string(content), fmt.Errorf("cannt send message, status: %d", rsp.StatusCode)
	}
	return string(content), nil
}

func newURLMessenger(cfg URLMsgConfig, rpl *caddy.Replacer) (*URLMsgConfig, error) {
	t, err := template.New("url").Parse(rpl.ReplaceKnown(cfg.URLTemplate, ""))
	if err != nil {
		return nil, fmt.Errorf("illegal template for url (%s): %v", cfg.URLTemplate, err)
	}
	var bodyTemplate *template.Template
	if cfg.BodyTemplate != "" {
		bodyTemplate, err = template.New("body").Parse(rpl.ReplaceKnown(cfg.BodyTemplate, ""))
		if err != nil {
			return nil, fmt.Errorf("illegal template for body (%s): %v", cfg.BodyTemplate, err)
		}
	}
	client := &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
			Proxy:             http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: cfg.Insecure,
			},
		},
	}
	cfg.client = client
	cfg.bodyTemplate = bodyTemplate
	cfg.urlTemplate = t
	for k, vs := range cfg.Headers {
		replv := make([]string, len(vs))
		for _, v := range vs {
			replv = append(replv, rpl.ReplaceKnown(v, ""))
		}
		cfg.Headers[k] = replv
	}
	return &cfg, nil
}

type SMTPMsgConfig struct {
	Host               string `json:"host"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify,omitempty"`
	User               string `json:"user"`
	Password           string `json:"password"`
	SSL                bool   `json:"ssl"`
	From               string `json:"from"`
	dialer             *mail.Dialer
}

func newSMTPMessenger(cfg SMTPMsgConfig, rpl *caddy.Replacer) (*SMTPMsgConfig, error) {
	h, ps, err := net.SplitHostPort(rpl.ReplaceKnown(cfg.Host, ""))
	if err != nil {
		return nil, fmt.Errorf("cannot parse host:port value %q: %w", cfg.Host, err)
	}
	p, _ := strconv.ParseInt(ps, 10, 0)
	d := mail.NewDialer(h, int(p), rpl.ReplaceKnown(cfg.User, ""), rpl.ReplaceKnown(cfg.Password, ""))
	if cfg.InsecureSkipVerify {
		d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	}
	// this is normally not needed, as most SMTP server use StartTLS instead
	d.SSL = cfg.SSL

	cfg.From = rpl.ReplaceKnown(cfg.From, "")

	// do not connect to the smtp server here; if the mail server is missing
	// i don't want my own service to fail when starting up. we will connect
	// lazy
	cfg.dialer = d
	return &cfg, nil
}

func (sm *SMTPMsgConfig) Send(lg *zap.Logger, a addressable, subject, shortmessage, mbody string) (string, error) {
	m := mail.NewMessage()
	if a.FromName != "" {
		m.SetAddressHeader("From", a.FromMail, a.FromName)
	} else {
		m.SetHeader("From", a.FromMail)
	}
	m.SetHeader("To", a.ToMail)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", mbody)

	if err := sm.dialer.DialAndSend(m); err != nil {
		return "", fmt.Errorf("cannot send email: %w", err)
	}
	return "mail sent", nil
}

type messageRSP struct {
	content string
	err     error
}
type messageRQ struct {
	*zap.Logger
	a        addressable
	subject  string
	message  string
	body     string
	response chan messageRSP
}

type messageSender struct {
	input    chan messageRQ
	throttle chan time.Time
	msg      messageTransport
}

func newMessageSender(lg *zap.Logger, cl clock.Clock, m messageTransport, rate time.Duration, burst int) *messageSender {
	return (&messageSender{msg: m, input: make(chan messageRQ)}).startLimiterWithBurst(lg, cl, rate, burst)
}

func (ms *messageSender) Send(lg *zap.Logger, a addressable, subject, shortmessage, body string) (string, error) {
	rq := messageRQ{Logger: lg, a: a, subject: subject, message: shortmessage, body: body, response: make(chan messageRSP)}
	ms.input <- rq
	rsp := <-rq.response
	close(rq.response)
	return rsp.content, rsp.err
}

func (ms *messageSender) startLimiterWithBurst(lg *zap.Logger, cl clock.Clock, rate time.Duration, burstLimit int) *messageSender {
	tick := cl.Ticker(rate)
	ms.throttle = make(chan time.Time, burstLimit)
	go ms.sendToBackend()
	go func() {
		defer tick.Stop()
		lg.Info("Message Limiter", zap.Duration("rate", rate), zap.Int("burst", burstLimit))
		for t := range tick.C {
			select {
			case ms.throttle <- t:
			default:
			}
		}
	}()
	return ms
}

func (ms *messageSender) sendToBackend() {
	for m := range ms.input {
		<-ms.throttle
		var mrsp messageRSP
		mrsp.content, mrsp.err = ms.msg.Send(m.Logger, m.a, m.subject, m.message, m.body)
		m.response <- mrsp
	}
}

func fromTransportSpec(_ *zap.Logger, b TypedPlugin) (messageTransport, error) {
	r := caddy.NewReplacer()
	switch b.Type {
	case valueURLMessenger:
		var d URLMsgConfig
		if err := json.Unmarshal(b.Spec, &d); err != nil {
			return nil, fmt.Errorf("cannot unmarshal url messenger: %w", err)
		}
		msg, err := newURLMessenger(d, r)
		if err != nil {
			return nil, err
		}
		return msg, nil
	case valueCommandMessenger:
		var d StdinMsgConfig
		if err := json.Unmarshal(b.Spec, &d); err != nil {
			return nil, fmt.Errorf("cannot unmarshal url messenger: %w", err)
		}
		return newStdin(d, r)
	case valueEMailMessenger:
		var d SMTPMsgConfig
		if err := json.Unmarshal(b.Spec, &d); err != nil {
			return nil, fmt.Errorf("cannot unmarshal smtp messenger: %w", err)
		}
		return newSMTPMessenger(d, r)
	default:
		return nil, fmt.Errorf("unknown messenger type: %q", b.Type)
	}

}

func fromTransportSpecs(log *zap.Logger, bks Plugins) (transporters, error) {
	res := make(transporters)
	for _, b := range bks {
		mt, err := fromTransportSpec(log, b)
		if err != nil {
			return nil, err
		}
		res[b.Name] = mt
	}

	return res, nil
}
