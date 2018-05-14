package connection

import "testing"

func Test_buildAccountName(t *testing.T) {
	type args struct {
		account    remoteAccount
		connection string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			"add name",
			args{remoteAccount{"foo", ""}, "bar"},
			"foo@bar",
		},
		{
			"replace name",
			args{remoteAccount{"foo", ""}, "ubuntu@bar"},
			"foo@bar",
		},
		{
			"add name FQDN",
			args{remoteAccount{"foo", ""}, "bar.example.com"},
			"foo@bar.example.com",
		},
		{
			"replace name FQDN",
			args{remoteAccount{"foo", ""}, "ubuntu@bar.example.com"},
			"foo@bar.example.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildAccountName(tt.args.account, tt.args.connection); got != tt.want {
				t.Errorf("buildAccountName() = %v, want %v", got, tt.want)
			}
		})
	}
}
