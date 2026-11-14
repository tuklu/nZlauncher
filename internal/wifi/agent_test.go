package wifi

import "testing"

func TestAgentPassesTheAnswerBack(t *testing.T) {
	a := &iwdAgent{ssid: "home", ask: func(prompt string) (string, error) {
		if prompt != "Passphrase for home" {
			t.Errorf("prompt %q", prompt)
		}
		return "hunter22", nil
	}}
	pass, err := a.RequestPassphrase("/net/connman/iwd/0/1/x_psk")
	if err != nil || pass != "hunter22" {
		t.Fatalf("got %q, %v", pass, err)
	}
	if a.canceled.Load() {
		t.Error("an answered prompt counted as canceled")
	}
}

func TestAgentClosedDialogCancels(t *testing.T) {
	a := &iwdAgent{ssid: "home", ask: func(string) (string, error) { return "", ErrCanceled }}
	if _, err := a.RequestPassphrase("/x"); err == nil || err.Name != "net.connman.iwd.Agent.Error.Canceled" {
		t.Fatalf("got %v", err)
	}
	if !a.canceled.Load() {
		t.Error("a closed dialog did not count as canceled")
	}
}

func TestAgentDeclinesEnterprise(t *testing.T) {
	a := &iwdAgent{ask: func(string) (string, error) { t.Fatal("asked for an enterprise secret"); return "", nil }}
	if _, _, err := a.RequestUserNameAndPassword("/x"); err == nil {
		t.Error("enterprise credentials were not declined")
	}
}
