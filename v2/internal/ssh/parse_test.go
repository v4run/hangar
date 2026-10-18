package ssh

import "testing"

func TestParseBasic(t *testing.T) {
	r, err := ParseSSHCommand("ssh deploy@web-1.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if r.Connection.Host != "web-1.example.com" || r.Connection.User != "deploy" {
		t.Fatalf("got %+v", r.Connection)
	}
	if r.Connection.Name != "web-1.example.com" {
		t.Fatalf("name: %q", r.Connection.Name)
	}
}

func TestParsePortAndKey(t *testing.T) {
	r, err := ParseSSHCommand(`ssh -p 2222 -i ~/.ssh/id_ed25519 admin@10.0.0.1`)
	if err != nil {
		t.Fatal(err)
	}
	c := r.Connection
	if c.Port != 2222 || c.IdentityFile != "~/.ssh/id_ed25519" || c.User != "admin" || c.Host != "10.0.0.1" {
		t.Fatalf("got %+v", c)
	}
}

func TestParseOptions(t *testing.T) {
	r, err := ParseSSHCommand(`ssh -o ForwardAgent=yes -o ServerAliveInterval=60 -A host`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Connection.SSHOptions == nil || r.Connection.SSHOptions.ForwardAgent == nil || !*r.Connection.SSHOptions.ForwardAgent {
		t.Fatalf("forward agent: %+v", r.Connection.SSHOptions)
	}
	if r.Connection.SSHOptions.ServerAliveInterval == nil || *r.Connection.SSHOptions.ServerAliveInterval != 60 {
		t.Fatalf("SAI: %+v", r.Connection.SSHOptions.ServerAliveInterval)
	}
}

func TestParseJumpHost(t *testing.T) {
	r, err := ParseSSHCommand("ssh -J bastion@10.0.0.1 target.local")
	if err != nil {
		t.Fatal(err)
	}
	if r.JumpHost != "bastion@10.0.0.1" {
		t.Fatalf("jump: %q", r.JumpHost)
	}
	if r.Connection.Host != "target.local" {
		t.Fatalf("host: %q", r.Connection.Host)
	}
}

func TestParseLocalForward(t *testing.T) {
	r, err := ParseSSHCommand(`ssh -L 8080:localhost:80 -L 5432:db.internal:5432 host`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Connection.SSHOptions.LocalForward) != 2 {
		t.Fatalf("LocalForward: %v", r.Connection.SSHOptions.LocalForward)
	}
}

func TestParseQuotedOption(t *testing.T) {
	r, err := ParseSSHCommand(`ssh -o "ProxyCommand=nc -x 127.0.0.1:9050 %h %p" host`)
	if err != nil {
		t.Fatal(err)
	}
	if v := r.Connection.SSHOptions.ExtraOptions["ProxyCommand"]; v != "nc -x 127.0.0.1:9050 %h %p" {
		t.Fatalf("ProxyCommand: %q", v)
	}
}

func TestParseNotSSH(t *testing.T) {
	if _, err := ParseSSHCommand("scp file host:/"); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseTrailingCommandIgnored(t *testing.T) {
	r, err := ParseSSHCommand(`ssh host tail -f /var/log/syslog`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Connection.Host != "host" {
		t.Fatalf("host: %q", r.Connection.Host)
	}
}
