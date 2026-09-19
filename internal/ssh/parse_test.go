package ssh

import "testing"

func TestParseBasic(t *testing.T) {
	c, err := ParseSSHCommand("ssh deploy@web-1.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if c.Host != "web-1.example.com" || c.User != "deploy" || c.Port != 22 {
		t.Fatalf("got %+v", c)
	}
	if c.Name != "web-1.example.com" {
		t.Fatalf("name: %q", c.Name)
	}
}

func TestParsePortAndKey(t *testing.T) {
	c, err := ParseSSHCommand(`ssh -p 2222 -i ~/.ssh/id_ed25519 admin@10.0.0.1`)
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != 2222 || c.IdentityFile != "~/.ssh/id_ed25519" || c.User != "admin" || c.Host != "10.0.0.1" {
		t.Fatalf("got %+v", c)
	}
}

func TestParsePortNoSpace(t *testing.T) {
	c, err := ParseSSHCommand("ssh -p2200 root@h")
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != 2200 {
		t.Fatalf("port %d", c.Port)
	}
}

func TestParseUserFlag(t *testing.T) {
	c, err := ParseSSHCommand("ssh -l alice example.com")
	if err != nil {
		t.Fatal(err)
	}
	if c.User != "alice" || c.Host != "example.com" {
		t.Fatalf("got %+v", c)
	}
}

func TestParseJumpHost(t *testing.T) {
	c, err := ParseSSHCommand("ssh -J bastion@10.0.0.1 target.local")
	if err != nil {
		t.Fatal(err)
	}
	if c.JumpHost != "bastion@10.0.0.1" || c.Host != "target.local" {
		t.Fatalf("got %+v", c)
	}
}

func TestParseOptions(t *testing.T) {
	c, err := ParseSSHCommand(`ssh -o ForwardAgent=yes -o ServerAliveInterval=60 -o Compression=no -A host`)
	if err != nil {
		t.Fatal(err)
	}
	if c.SSHOptions == nil {
		t.Fatal("expected SSHOptions")
	}
	if c.SSHOptions.ForwardAgent == nil || !*c.SSHOptions.ForwardAgent {
		t.Fatalf("ForwardAgent: %+v", c.SSHOptions.ForwardAgent)
	}
	if c.SSHOptions.ServerAliveInterval == nil || *c.SSHOptions.ServerAliveInterval != 60 {
		t.Fatalf("SAI: %+v", c.SSHOptions.ServerAliveInterval)
	}
	if c.SSHOptions.Compression == nil || *c.SSHOptions.Compression {
		t.Fatalf("Compression: %+v", c.SSHOptions.Compression)
	}
}

func TestParseLocalForward(t *testing.T) {
	c, err := ParseSSHCommand(`ssh -L 8080:localhost:80 -L 5432:db.internal:5432 host`)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.SSHOptions.LocalForward) != 2 || c.SSHOptions.LocalForward[0] != "8080:localhost:80" {
		t.Fatalf("LocalForward: %v", c.SSHOptions.LocalForward)
	}
}

func TestParseUnknownFlagsSkipped(t *testing.T) {
	c, err := ParseSSHCommand("ssh -F /etc/ssh/config -v -q host")
	if err != nil {
		t.Fatal(err)
	}
	if c.Host != "host" {
		t.Fatalf("host: %q", c.Host)
	}
}

func TestParseQuotedOption(t *testing.T) {
	c, err := ParseSSHCommand(`ssh -o "ProxyCommand=nc -x 127.0.0.1:9050 %h %p" host`)
	if err != nil {
		t.Fatal(err)
	}
	if v := c.SSHOptions.ExtraOptions["ProxyCommand"]; v != "nc -x 127.0.0.1:9050 %h %p" {
		t.Fatalf("ProxyCommand: %q", v)
	}
}

func TestParseTrailingCommandIgnored(t *testing.T) {
	c, err := ParseSSHCommand(`ssh host tail -f /var/log/syslog`)
	if err != nil {
		t.Fatal(err)
	}
	if c.Host != "host" {
		t.Fatalf("host: %q", c.Host)
	}
}

func TestParseNotSSH(t *testing.T) {
	if _, err := ParseSSHCommand("scp file host:/"); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseUnterminatedQuote(t *testing.T) {
	if _, err := ParseSSHCommand(`ssh -o "Foo=bar host`); err == nil {
		t.Fatal("expected error")
	}
}
