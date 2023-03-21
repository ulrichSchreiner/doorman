package doorman

import (
	"sync"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"go.uber.org/zap"
)

func Test_memstore_TTL(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		getkey   string
		value    string
		duration time.Duration
		putErr   bool
		getErr   bool
		deleted  bool
		skip     time.Duration
	}{
		{
			name:     "simple put",
			key:      "mykey",
			value:    "myvalue",
			duration: 1 * time.Hour,
			putErr:   false,
			getErr:   false,
			skip:     0,
		},
		{
			name:     "simple put with timeout",
			key:      "mykey",
			value:    "",
			duration: 1 * time.Hour,
			putErr:   false,
			getErr:   true,
			deleted:  true,
			skip:     2 * time.Hour,
		},
		{
			name:     "simple put wrong get-key",
			key:      "mykey",
			getkey:   "mywrongkey",
			value:    "",
			duration: 1 * time.Hour,
			putErr:   false,
			getErr:   true,
			skip:     2 * time.Hour,
		},
	}
	lg := zap.NewNop()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := clock.NewMock()
			ms := newMemstore(mock, StoreSettings{})
			err := ms.PutTTL(lg, tt.key, tt.value, tt.duration)
			if (err != nil) != tt.putErr {
				t.Errorf("memstore.Put() returned error: %v", err)
			}
			if tt.skip > 0 {
				mock.Add(tt.skip)
			}
			getkey := tt.key
			if tt.getkey != "" {
				getkey = tt.getkey
			}
			v, err := ms.GetTTL(lg, getkey)
			if (err != nil) != tt.getErr {
				t.Errorf("memstore.Get() returned error: %v", err)
			}
			if v != tt.value {
				t.Errorf("memstore.Get() returned %v, want %v", v, tt.value)
			}
			if tt.deleted {
				if _, has := ms.data[getkey]; has {
					t.Errorf("memstore.Get() should have deleted the key: %v", getkey)
				}
			}
		})
	}
}

func Test_memstore_GetPut(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		getkey string
		value  string
		putErr bool
		getErr bool
		delete bool
	}{
		{
			name:   "simple put",
			key:    "mykey",
			value:  "myvalue",
			putErr: false,
			getErr: false,
		},
		{
			name:   "deleted key",
			key:    "mykey",
			value:  "myvalue",
			putErr: false,
			getErr: false,
			delete: true,
		},
		{
			name:   "simple put wrong get-key",
			key:    "mykey",
			getkey: "mywrongkey",
			value:  "",
			putErr: false,
			getErr: true,
		},
	}
	lg := zap.NewNop()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := clock.NewMock()
			ms := newMemstore(mock, StoreSettings{})
			err := ms.Put(lg, tt.key, tt.value)
			if (err != nil) != tt.putErr {
				t.Errorf("memstore.Put() returned error: %v", err)
			}
			getkey := tt.key
			if tt.getkey != "" {
				getkey = tt.getkey
			}
			v, err := ms.Get(lg, getkey)
			if (err != nil) != tt.getErr {
				t.Errorf("memstore.Get() returned error: %v", err)
			}
			if v != tt.value {
				t.Errorf("memstore.Get() returned %v, want %v", v, tt.value)
			}
			if tt.delete {
				ms.Del(lg, getkey)
				if has := ms.Has(lg, getkey); has {
					t.Errorf("memstore.Del() should have deleted the key: %v", getkey)
				}
			}
		})
	}
}

func Test_YesNoWaiter(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		yes  bool
	}{
		{
			name: "say yes",
			cmd:  "yes",
			yes:  true,
		},
		{
			name: "say YES",
			cmd:  "YES",
			yes:  true,
		},
		{
			name: "say no",
			cmd:  "no",
			yes:  false,
		},
		{
			name: "say random",
			cmd:  "random",
			yes:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newYesNo()
			cmd := yesno(tt.cmd)
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				yn, err := w.WaitFor()
				if err != nil {
					t.Errorf("wait returned error: %v", err)
					return
				}
				if *yn != cmd {
					t.Errorf("wait returned %v, want %v", yn, cmd)
				}
				if yn.Yes() != tt.yes {
					t.Errorf("yes returned %v, want %v", yn.Yes(), tt.yes)
				}
			}()
			w.Say(cmd)
			wg.Wait()
		})
	}
}
