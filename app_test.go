package doorman

import "testing"

func Test_spacedToken(t *testing.T) {
	type args struct {
		spacer string
		token  string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "without spacing",
			args: args{
				spacer: "",
				token:  "123123",
			},
			want: "123123",
		},
		{
			name: "spacer with blanks",
			args: args{
				spacer: " ",
				token:  "123123",
			},
			want: "1 2 3 1 2 3",
		},
		{
			name: "spacer with more chars",
			args: args{
				spacer: "-123",
				token:  "123123",
			},
			want: "1-2-3-1-2-3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := spacedToken(tt.args.spacer, tt.args.token); got != tt.want {
				t.Errorf("spacedToken() = %v, want %v", got, tt.want)
			}
		})
	}
}
