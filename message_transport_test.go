package doorman

import (
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/caddyserver/caddy/v2"
	"go.uber.org/zap"
)

type emptyTransport struct {
	res   string
	err   error
	count int
}

func (et *emptyTransport) Send(lg *zap.Logger, a addressable, subject, message, body string) (string, error) {
	et.count--
	return et.res, et.err
}

func Test_messageSender_startLimiterWithBurst(t *testing.T) {
	tests := []struct {
		name   string
		count  int
		burst  int
		dur    time.Duration
		skip   time.Duration
		expect int
	}{
		{
			name:   "all sends within duration",
			count:  3,
			burst:  20,
			dur:    time.Second,
			skip:   time.Second * 3,
			expect: 0,
		},
		{
			name:   "only some sends within duration",
			count:  5,
			burst:  20,
			dur:    time.Second,
			skip:   time.Second * 3,
			expect: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cl := clock.NewMock()
			tr := &emptyTransport{
				res:   "ok",
				err:   nil,
				count: tt.count,
			}
			ms := newMessageSender(zap.NewNop(), cl, tr, tt.dur, tt.burst)
			for i := 0; i < tr.count; i++ {
				go func(t *testing.T) {
					_, e := ms.Send(zap.NewNop(), addressable{}, "subject", "message", "body")
					if e != nil {
						t.Errorf("send message returned error: %v", e)
					}
				}(t)
			}
			cl.Add(tt.skip)
			// do some sleep, so golang goroutines can do their work
			time.Sleep(time.Millisecond * 300)
			if tr.count != tt.expect {
				t.Errorf("not all messages were sent: count: %d", tr.count)
			}
		})
	}
}

func TestURLMessenger(t *testing.T) {
	recipient, subject, mymessage, body := "recipient", "mysubject", "mymessage", "mybody"
	a := addressable{
		ToMail: recipient,
	}

	tests := []struct {
		name     string
		path     string
		header   http.Header
		body     string
		user     string
		password string
		wantBody string
		urlRes   string
		method   string
		wantErr  bool
	}{
		{
			name:    "successful url get",
			path:    "/mypath",
			urlRes:  "OK",
			method:  "GET",
			wantErr: false,
		},
		{
			name:    "successful with header",
			path:    "/mypath",
			urlRes:  "OK",
			method:  "GET",
			header:  http.Header{"X-My-Header": []string{"a"}},
			wantErr: false,
		},
		{
			name:     "successful with basic auth",
			path:     "/mypath",
			urlRes:   "OK",
			method:   "GET",
			user:     "donald",
			password: "duck",
			wantErr:  false,
		},
		{
			name:     "successful url post",
			path:     "/mypath",
			urlRes:   "OK",
			method:   "POST",
			body:     "my test body \\{\\{.message}} to \\{\\{.tomail}}",
			wantBody: "my test body " + mymessage + " to " + recipient,
			wantErr:  false,
		},
		{
			name:    "unsuccessful url post",
			path:    "/mypath",
			urlRes:  "OK",
			method:  "POST",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				if req.URL.String() != tt.path {
					t.Errorf("want path %q but got %q", tt.path, req.URL.String())
				}
				if req.Method != tt.method {
					t.Errorf("want method %q but got %q", tt.method, req.Method)
				}
				if tt.wantBody != "" {
					dat, _ := io.ReadAll(req.Body)
					if string(dat) != tt.wantBody {
						t.Errorf("want body %q but got %q", tt.wantBody, dat)
					}
				}
				if len(tt.header) > 0 {
					for k, v := range tt.header {
						hv := req.Header[k]
						if !reflect.DeepEqual(v, hv) {
							t.Errorf("the value of %q is %q, but wanted %q", k, hv, v)
						}
					}
				}
				if tt.user != "" {
					u, p, _ := req.BasicAuth()
					if tt.user != u {
						t.Errorf("want username %s but got: %s", tt.user, u)
					}
					if p != tt.password {
						t.Errorf("want password %s but got: %s", tt.password, p)
					}
				}
				if tt.wantErr {
					rw.WriteHeader(http.StatusInternalServerError)
				}
				_, _ = rw.Write([]byte(tt.urlRes))
			}))
			defer server.Close()

			um, err := newURLMessenger(URLMsgConfig{
				Method:       tt.method,
				URLTemplate:  server.URL + tt.path,
				BodyTemplate: tt.body,
				Headers:      tt.header,
				AuthUser:     tt.user,
				AuthPassword: tt.password,
				Insecure:     false,
			}, caddy.NewReplacer())
			if err != nil {
				t.Errorf("cannot create url messenger: %v", err)
			}
			res, err := um.Send(zap.NewNop(), a, subject, mymessage, body)
			if (err != nil) != tt.wantErr {
				t.Errorf("cannot send message: %v", err)
			} else {
				if res != tt.urlRes {
					t.Errorf("result is %q, but want %q", res, tt.urlRes)
				}
			}
		})
	}
}
