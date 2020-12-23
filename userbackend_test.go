package doorman

import (
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"go.uber.org/zap"
)

func generate_test_script(t *testing.T, content string) string {
	tmpfile, err := ioutil.TempFile("", "script")
	if err != nil {
		t.Fatal(err)
	}
	_ = tmpfile.Chmod(0755)
	fmt.Fprintf(tmpfile, `#!/bin/bash
echo %q
`, content)
	_ = tmpfile.Close()
	return tmpfile.Name()
}

func Test_userSearchCommand_Search(t *testing.T) {
	type fields struct {
		Content  string
		Args     []string
		UseStdin bool
	}
	type args struct {
		uid string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *UserEntry
		wantErr bool
	}{
		{
			name: "legal json content",
			fields: fields{
				Content: `{"name":"Donald Duck","uid":"ddk","mobile":"123123123","email":"dd@donald.duck"}`,
			},
			args: args{
				uid: "test",
			},
			want: &UserEntry{
				Name:   "Donald Duck",
				UID:    "ddk",
				Mobile: "123123123",
				EMail:  "dd@donald.duck",
			},
			wantErr: false,
		},
		{
			name: "illegal json content",
			fields: fields{
				Content: `[{"name":"Donald Duck","uid":"ddk","mobile":"123123123","email":"dd@donald.duck"}]`,
			},
			args: args{
				uid: "test",
			},
			wantErr: true,
		},
		{
			name: "userid whould be the parameter",
			fields: fields{
				Content: `{"name":"Donald Duck","uid":"$1","mobile":"123123123","email":"dd@donald.duck"}`,
			},
			args: args{
				uid: "ddk",
			},
			want: &UserEntry{
				Name:   "Donald Duck",
				UID:    "ddk",
				Mobile: "123123123",
				EMail:  "dd@donald.duck",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := generate_test_script(t, tt.fields.Content)
			defer os.Remove(cmd)
			usc := &userSearchCommand{
				Command:  cmd,
				Args:     tt.fields.Args,
				UseStdin: tt.fields.UseStdin,
			}
			got, err := usc.Search(zap.NewNop(), tt.args.uid)
			if (err != nil) != tt.wantErr {
				t.Errorf("userSearchCommand.Search() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("userSearchCommand.Search() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_userlistBackend_Search(t *testing.T) {
	userlist := []UserEntry{
		UserEntry{
			Name:   "Donald Duck",
			UID:    "ddk",
			Mobile: "123123123",
			EMail:  "dd@donald.duck",
		},
		UserEntry{
			Name:   "Daisy Duck",
			UID:    "dsdk",
			Mobile: "123123123",
			EMail:  "dsdk@daisy.duck",
		},
		UserEntry{
			Name:   "Dagobert Duck",
			UID:    "dgdk",
			Mobile: "123123123",
			EMail:  "dgdk@dagobert.duck",
		},
	}

	type args struct {
		uid string
	}
	tests := []struct {
		name    string
		ulb     userlistBackend
		args    args
		want    *UserEntry
		wantErr bool
	}{
		{
			name: "name in list",
			ulb:  userlist,
			args: args{
				uid: "ddk",
			},
			want: &userlist[0],
		},
		{
			name: "name not in list",
			ulb:  userlist,
			args: args{
				uid: "abc",
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.ulb.Search(zap.NewNop(), tt.args.uid)
			if (err != nil) != tt.wantErr {
				t.Errorf("userlistBackend.Search() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("userlistBackend.Search() = %v, want %v", got, tt.want)
			}
		})
	}
}
