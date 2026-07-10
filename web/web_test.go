package web

import (
	"io/fs"
	"strings"
	"testing"
)

func TestNavigationModuleExportsAdminAndAccountInitializers(t *testing.T) {
	nav, err := fs.ReadFile(Static, "static/core/nav.js")
	if err != nil {
		t.Fatal(err)
	}
	account, err := fs.ReadFile(Static, "static/account/app.js")
	if err != nil {
		t.Fatal(err)
	}

	for _, exported := range []string{"export function initSectionNav", "export function initViewNav"} {
		if !strings.Contains(string(nav), exported) {
			t.Fatalf("navigation module is missing %q", exported)
		}
	}
	if !strings.Contains(string(account), `import { initSectionNav } from "../core/nav.js"`) {
		t.Fatal("account application no longer imports the compatible section navigator")
	}
}
