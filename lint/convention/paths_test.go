package convention

import "testing"

func TestParsePkgPath(t *testing.T) {
	tests := []struct {
		name       string
		pkgPath    string
		wantNil    bool
		wantDomain string
		wantSubPkg string
		wantKind   PackageKind
	}{
		{
			name:       "public command package",
			pkgPath:    "github.com/myproject/ddd/order/command",
			wantDomain: "order",
			wantSubPkg: "command",
			wantKind:   KindPublic,
		},
		{
			name:       "public query package",
			pkgPath:    "github.com/myproject/ddd/order/query",
			wantDomain: "order",
			wantSubPkg: "query",
			wantKind:   KindPublic,
		},
		{
			name:       "public event package",
			pkgPath:    "github.com/myproject/ddd/order/event",
			wantDomain: "order",
			wantSubPkg: "event",
			wantKind:   KindPublic,
		},
		{
			name:       "internal domain package",
			pkgPath:    "github.com/myproject/ddd/order/domain",
			wantDomain: "order",
			wantSubPkg: "domain",
			wantKind:   KindInternal,
		},
		{
			name:       "internal service package",
			pkgPath:    "github.com/myproject/ddd/order/service",
			wantDomain: "order",
			wantSubPkg: "service",
			wantKind:   KindInternal,
		},
		{
			name:       "internal repository package",
			pkgPath:    "github.com/myproject/ddd/order/repository",
			wantDomain: "order",
			wantSubPkg: "repository",
			wantKind:   KindInternal,
		},
		{
			name:       "wire package",
			pkgPath:    "github.com/myproject/ddd/order/wire",
			wantDomain: "order",
			wantSubPkg: "wire",
			wantKind:   KindWire,
		},
		{
			name:       "domain root is internal",
			pkgPath:    "github.com/myproject/ddd/order",
			wantDomain: "order",
			wantSubPkg: "",
			wantKind:   KindInternal,
		},
		{
			name:       "nested subpackage under domain",
			pkgPath:    "github.com/myproject/ddd/order/domain/model",
			wantDomain: "order",
			wantSubPkg: "domain/model",
			wantKind:   KindInternal,
		},
		{
			name:       "nested subpackage under command",
			pkgPath:    "github.com/myproject/ddd/order/command/sub",
			wantDomain: "order",
			wantSubPkg: "command/sub",
			wantKind:   KindPublic,
		},
		{
			name:    "non-ddd package",
			pkgPath: "github.com/myproject/api/handler",
			wantNil: true,
		},
		{
			name:    "ddd alone no domain",
			pkgPath: "github.com/myproject/ddd",
			wantNil: true,
		},
		{
			name:       "other subpackage",
			pkgPath:    "github.com/myproject/ddd/order/utils",
			wantDomain: "order",
			wantSubPkg: "utils",
			wantKind:   KindOther,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParsePkgPath(tt.pkgPath)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("ParsePkgPath(%q) = %+v, want nil", tt.pkgPath, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("ParsePkgPath(%q) = nil, want non-nil", tt.pkgPath)
			}
			if got.DomainName != tt.wantDomain {
				t.Errorf("DomainName = %q, want %q", got.DomainName, tt.wantDomain)
			}
			if got.SubPkg != tt.wantSubPkg {
				t.Errorf("SubPkg = %q, want %q", got.SubPkg, tt.wantSubPkg)
			}
			if got.Kind != tt.wantKind {
				t.Errorf("Kind = %v, want %v", got.Kind, tt.wantKind)
			}
		})
	}
}

func TestSameDomain(t *testing.T) {
	tests := []struct {
		name string
		pkg1 string
		pkg2 string
		want bool
	}{
		{
			name: "same domain",
			pkg1: "github.com/myproject/ddd/order/command",
			pkg2: "github.com/myproject/ddd/order/domain",
			want: true,
		},
		{
			name: "different domain",
			pkg1: "github.com/myproject/ddd/order/command",
			pkg2: "github.com/myproject/ddd/inventory/domain",
			want: false,
		},
		{
			name: "one non-ddd",
			pkg1: "github.com/myproject/ddd/order/command",
			pkg2: "github.com/myproject/api/handler",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SameDomain(tt.pkg1, tt.pkg2); got != tt.want {
				t.Errorf("SameDomain(%q, %q) = %v, want %v", tt.pkg1, tt.pkg2, got, tt.want)
			}
		})
	}
}
