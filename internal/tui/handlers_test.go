package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/zalando/go-keyring"

	"github.com/v4run/hangar/internal/config"
)

func newTestModel(t *testing.T) Model {
	t.Helper()
	return NewModel(&config.HangarConfig{}, config.DefaultGlobalConfig(), t.TempDir(), false)
}

func TestSaveFormWarnsWhenKeyringUnavailable(t *testing.T) {
	keyring.MockInitWithError(errors.New("The name is not activatable"))
	t.Cleanup(keyring.MockInit)

	m := newTestModel(t)
	m.form = formAdd
	m.formFields = make([]string, fieldAdvancedCount)
	m.formFields[fieldName] = "srv"
	m.formFields[fieldHost] = "10.0.0.1"
	m.formFields[fieldPort] = "22"
	m.formFields[fieldUser] = "root"
	m.formFields[fieldPassword] = "hunter2"

	got, _ := m.saveForm()
	m = got.(Model)

	if m.form != formNone {
		t.Fatalf("form should close (connection was saved), got form=%v", m.form)
	}
	if len(m.cfg.Connections) != 1 {
		t.Fatalf("expected connection saved, got %d", len(m.cfg.Connections))
	}
	if m.activeToast == nil || m.activeToast.kind != toastErr {
		t.Fatalf("expected error toast, got %+v", m.activeToast)
	}
	if !strings.Contains(m.activeToast.text, "password not stored: keyring unavailable") {
		t.Fatalf("unexpected toast text: %q", m.activeToast.text)
	}
}

func TestSaveDBFormWarnsWhenKeyringUnavailable(t *testing.T) {
	keyring.MockInitWithError(errors.New("The name is not activatable"))
	t.Cleanup(keyring.MockInit)

	m := newTestModel(t)
	m.form = formAddDatabase
	m.formFields = make([]string, dbFieldCount)
	m.formFields[dbFieldName] = "pg"
	m.formFields[dbFieldEngine] = "postgres"
	m.formFields[dbFieldHost] = "10.0.0.1"
	m.formFields[dbFieldPort] = "5432"
	m.formFields[dbFieldPassword] = "hunter2"

	got, _ := m.saveDBForm()
	m = got.(Model)

	if m.form != formNone {
		t.Fatalf("form should close (database was saved), got form=%v", m.form)
	}
	if len(m.cfg.Databases) != 1 {
		t.Fatalf("expected database saved, got %d", len(m.cfg.Databases))
	}
	if m.activeToast == nil || m.activeToast.kind != toastErr {
		t.Fatalf("expected error toast, got %+v", m.activeToast)
	}
	if !strings.Contains(m.activeToast.text, "password not stored: keyring unavailable") {
		t.Fatalf("unexpected toast text: %q", m.activeToast.text)
	}
}

func TestSaveFormOKToastWhenKeyringWorks(t *testing.T) {
	keyring.MockInit()

	m := newTestModel(t)
	m.form = formAdd
	m.formFields = make([]string, fieldAdvancedCount)
	m.formFields[fieldName] = "srv"
	m.formFields[fieldHost] = "10.0.0.1"
	m.formFields[fieldPort] = "22"
	m.formFields[fieldUser] = "root"
	m.formFields[fieldPassword] = "hunter2"

	got, _ := m.saveForm()
	m = got.(Model)

	if m.activeToast == nil || m.activeToast.kind != toastOK {
		t.Fatalf("expected ok toast, got %+v", m.activeToast)
	}
	pw, err := config.GetPassword(m.cfg.Connections[0].ID.String())
	if err != nil || pw != "hunter2" {
		t.Fatalf("password not in keyring: %q %v", pw, err)
	}
}

func groupedCfg(groups ...string) *config.HangarConfig {
	cfg := &config.HangarConfig{Groups: groups}
	for _, g := range groups {
		_ = cfg.Add(config.Connection{Name: g + "-1", Host: "h", User: "u", Port: 22, Group: g})
	}
	return cfg
}

func TestNewModelLoadsCollapsedGroups(t *testing.T) {
	dir := t.TempDir()
	if err := config.SaveState(dir, &config.UIState{CollapsedGroups: []string{"prod"}}); err != nil {
		t.Fatal(err)
	}
	m := NewModel(groupedCfg("prod", "dev"), config.DefaultGlobalConfig(), dir, false)
	if !m.collapsed["prod"] {
		t.Fatal("prod should start collapsed")
	}
	if m.collapsed["dev"] {
		t.Fatal("dev should start expanded")
	}
}

func TestToggleGroupPersistsCollapsedState(t *testing.T) {
	dir := t.TempDir()
	m := NewModel(groupedCfg("prod"), config.DefaultGlobalConfig(), dir, false)
	m.cursor = 0 // the "prod" group header

	got, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = got.(Model)
	st, err := config.LoadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.CollapsedGroups) != 1 || st.CollapsedGroups[0] != "prod" {
		t.Fatalf("expected [prod] persisted, got %v", st.CollapsedGroups)
	}

	got, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = got.(Model)
	st, err = config.LoadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.CollapsedGroups) != 0 {
		t.Fatalf("expected nothing persisted after re-expand, got %v", st.CollapsedGroups)
	}
}

func TestRenameGroupPersistsCollapsedState(t *testing.T) {
	dir := t.TempDir()
	m := NewModel(groupedCfg("prod"), config.DefaultGlobalConfig(), dir, false)
	m.collapsed["prod"] = true
	m.form = formEditGroup
	m.formTargetGroup = "prod"
	m.editInput.SetValue("production")

	got, _ := m.handleEditGroupInput(tea.KeyMsg{Type: tea.KeyEnter})
	m = got.(Model)
	st, err := config.LoadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.CollapsedGroups) != 1 || st.CollapsedGroups[0] != "production" {
		t.Fatalf("expected [production] persisted, got %v", st.CollapsedGroups)
	}
}

func TestDeleteGroupPersistsCollapsedState(t *testing.T) {
	dir := t.TempDir()
	if err := config.SaveState(dir, &config.UIState{CollapsedGroups: []string{"prod"}}); err != nil {
		t.Fatal(err)
	}
	m := NewModel(groupedCfg("prod"), config.DefaultGlobalConfig(), dir, false)
	m.form = formDeleteGroup
	m.formTargetGroup = "prod"

	got, _ := m.handleDeleteGroupConfirm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = got.(Model)
	st, err := config.LoadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.CollapsedGroups) != 0 {
		t.Fatalf("expected deleted group removed from state, got %v", st.CollapsedGroups)
	}
}
