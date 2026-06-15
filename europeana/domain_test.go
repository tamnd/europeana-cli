package europeana

import (
	"testing"
)

// These tests are offline: they exercise the URI driver's pure string functions.
// The client's HTTP behaviour is covered in europeana_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "europeana" {
		t.Errorf("Scheme = %q, want europeana", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "europeana" {
		t.Errorf("Identity.Binary = %q, want europeana", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	typ, id, err := Domain{}.Classify("/91619/SMVK_EM_objekt_1059045")
	if err != nil {
		t.Fatalf("Classify error: %v", err)
	}
	if typ != "item" {
		t.Errorf("type = %q, want item", typ)
	}
	if id != "/91619/SMVK_EM_objekt_1059045" {
		t.Errorf("id = %q, want /91619/SMVK_EM_objekt_1059045", id)
	}
}

func TestLocate(t *testing.T) {
	got, err := Domain{}.Locate("item", "/91619/SMVK_EM_objekt_1059045")
	if err != nil {
		t.Fatalf("Locate error: %v", err)
	}
	want := "https://www.europeana.eu/item/91619/SMVK_EM_objekt_1059045"
	if got != want {
		t.Errorf("Locate = %q, want %q", got, want)
	}
}

func TestLocateUnknownType(t *testing.T) {
	_, err := Domain{}.Locate("unknown", "foo")
	if err == nil {
		t.Error("expected error for unknown type")
	}
}
