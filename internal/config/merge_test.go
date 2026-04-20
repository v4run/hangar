package config

import "testing"

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }

func TestMergeSSHOptionsNilGlobal(t *testing.T) {
	local := &SSHOptions{ForwardAgent: boolPtr(true)}
	result := MergeSSHOptions(nil, local)
	if result.ForwardAgent == nil || !*result.ForwardAgent {
		t.Fatal("expected ForwardAgent=true from local")
	}
}

func TestMergeSSHOptionsNilLocal(t *testing.T) {
	global := &SSHOptions{Compression: boolPtr(true)}
	result := MergeSSHOptions(global, nil)
	if result.Compression == nil || !*result.Compression {
		t.Fatal("expected Compression=true from global")
	}
}

func TestMergeSSHOptionsBothNil(t *testing.T) {
	result := MergeSSHOptions(nil, nil)
	if result.ForwardAgent != nil || result.Compression != nil {
		t.Fatal("expected all nil fields")
	}
}

func TestMergeSSHOptionsLocalOverridesGlobal(t *testing.T) {
	global := &SSHOptions{
		ForwardAgent:        boolPtr(true),
		Compression:         boolPtr(false),
		ServerAliveInterval: intPtr(30),
		StrictHostKeyCheck:  "yes",
		LocalForward:        []string{"8080:localhost:80"},
		EnvVars:             map[string]string{"ENV": "prod"},
		ExtraOptions:        map[string]string{"TCPKeepAlive": "yes"},
	}
	local := &SSHOptions{
		ForwardAgent:       boolPtr(false),
		Compression:        boolPtr(true),
		StrictHostKeyCheck: "no",
		LocalForward:       []string{"9090:localhost:90"},
		EnvVars:            map[string]string{"APP": "myapp"},
	}
	result := MergeSSHOptions(global, local)
	if *result.ForwardAgent != false {
		t.Fatal("local should override ForwardAgent")
	}
	if *result.Compression != true {
		t.Fatal("local should override Compression")
	}
	if *result.ServerAliveInterval != 30 {
		t.Fatal("global ServerAliveInterval should carry through")
	}
	if result.StrictHostKeyCheck != "no" {
		t.Fatal("local should override StrictHostKeyCheck")
	}
	if len(result.LocalForward) != 1 || result.LocalForward[0] != "9090:localhost:90" {
		t.Fatalf("expected local forwards only, got %v", result.LocalForward)
	}
	if result.EnvVars["ENV"] != "prod" {
		t.Fatal("global ENV should carry through")
	}
	if result.EnvVars["APP"] != "myapp" {
		t.Fatal("local APP should be present")
	}
	if result.ExtraOptions["TCPKeepAlive"] != "yes" {
		t.Fatal("global ExtraOptions should carry through")
	}
}
