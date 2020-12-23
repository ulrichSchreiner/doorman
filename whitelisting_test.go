package doorman

import (
	"net"
	"reflect"
	"testing"

	"go.uber.org/zap"
)

func createNetworks(nw ...string) Networks {
	res := make(Networks, 0)
	for _, n := range nw {
		_, netw, _ := net.ParseCIDR(n)
		res = append(res, netw)
	}
	return res
}

func compareNetworks(t *testing.T, got, want Networks) {
	if len(got) != len(want) {
		t.Errorf("got: %v, want %v", got, want)
		return
	}
	for i := range got {
		nw1 := got[i]
		nw2 := want[i]
		if nw1.String() != nw2.String() {
			t.Errorf("got: %v, want %v", got, want)
			return
		}
	}
}

func Test_parseStaticList(t *testing.T) {
	type args struct {
		ips []string
	}
	tests := []struct {
		name    string
		args    args
		want    IPMap
		want1   Networks
		wantErr bool
	}{
		{
			name: "single ip",
			args: args{
				ips: []string{"1.2.3.4"},
			},
			want:    IPMap{"1.2.3.4": true},
			want1:   make(Networks, 0),
			wantErr: false,
		},
		{
			name: "illegal ip",
			args: args{
				ips: []string{"a.2.3.4"},
			},
			want:    nil,
			want1:   nil,
			wantErr: true,
		},
		{
			name: "single network",
			args: args{
				ips: []string{"1.2.3.4/8"},
			},
			want:    make(IPMap),
			want1:   createNetworks("1.0.0.0/8"),
			wantErr: false,
		},
		{
			name: "network and ip",
			args: args{
				ips: []string{"1.2.3.4/8", "2.3.4.5"},
			},
			want:    IPMap{"2.3.4.5": true},
			want1:   createNetworks("1.0.0.0/8"),
			wantErr: false,
		},
		{
			name: "network with 32 mask",
			args: args{
				ips: []string{"1.2.3.4/32"},
			},
			want:    IPMap{"1.2.3.4": true},
			want1:   make(Networks, 0),
			wantErr: false,
		},
		{
			name: "network and ip with illegal entry",
			args: args{
				ips: []string{"1.2.3.4/8", "2.3.4.5", "1.2.3.4/33"},
			},
			want:    nil,
			want1:   nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := parseStaticList(tt.args.ips)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseStaticList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseStaticList() map: got = %v, want %v", got, tt.want)
			}
			compareNetworks(t, got1, tt.want1)
		})
	}
}

func Test_isAllowed(t *testing.T) {
	type args struct {
		list     staticWhiteList
		clientip string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "empty store",
			args: args{
				clientip: "127.0.0.1",
			},
			want: false,
		},
		{
			name: "illegal IP",
			args: args{
				clientip: "a.b.c.d",
			},
			want: false,
		},
		{
			name: "ip is allowed",
			args: args{
				list:     []string{"127.0.0.1"},
				clientip: "127.0.0.1",
			},
			want: true,
		},
		{
			name: "ip is in network",
			args: args{
				list:     []string{"127.0.0.0/24"},
				clientip: "127.0.0.1",
			},
			want: true,
		},
		{
			name: "ip is in network and explicit allowed",
			args: args{
				list:     []string{"127.0.0.1", "127.0.0.0/24"},
				clientip: "127.0.0.1",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wl, err := tt.args.list.Fetch(zap.NewNop())
			if (err != nil) != tt.wantErr {
				t.Errorf("Fetch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got := wl.IsAllowed(zap.NewNop(), tt.args.clientip); got != tt.want {
				t.Errorf("isAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}
