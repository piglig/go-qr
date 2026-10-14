package main

import "testing"

func TestVersion(t *testing.T) {
	if v := version(); v == "" {
		t.Error("empty version")
	}
	defer func(v string) { buildVersion = v }(buildVersion)
	buildVersion = "v1.2.3"
	if v := version(); v != "v1.2.3" {
		t.Errorf("version = %q, want v1.2.3", v)
	}
}
