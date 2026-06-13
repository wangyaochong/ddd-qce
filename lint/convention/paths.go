package convention

import "strings"

type PackageKind int

const (
	KindOther    PackageKind = iota
	KindPublic               // command, query, event
	KindInternal             // domain, service, repository, or domain root
	KindWire                 // wire
)

type PkgInfo struct {
	DDDPrefix  string // everything before "ddd/" in the path
	ModuleName string // e.g., "order_service", "billing"
	DomainName string // e.g., "order"
	SubPkg     string // e.g., "command", "domain/model"
	Kind       PackageKind
}

func ParsePkgPath(pkgPath string) *PkgInfo {
	parts := strings.Split(pkgPath, "/")
	dddIdx := -1
	for i, p := range parts {
		if p == "ddd" {
			dddIdx = i
			break
		}
	}
	if dddIdx < 0 || dddIdx+1 >= len(parts) {
		return nil
	}

	prefix := strings.Join(parts[:dddIdx], "/")
	moduleName := parts[dddIdx-1]
	domainName := parts[dddIdx+1]

	var subPkg string
	if dddIdx+2 < len(parts) {
		subPkg = strings.Join(parts[dddIdx+2:], "/")
	}

	kind := classifySubPkg(subPkg)

	return &PkgInfo{
		DDDPrefix:  prefix,
		ModuleName: moduleName,
		DomainName: domainName,
		SubPkg:     subPkg,
		Kind:       kind,
	}
}

func classifySubPkg(subPkg string) PackageKind {
	if subPkg == "" {
		return KindInternal
	}

	firstSeg := subPkg
	if idx := strings.Index(subPkg, "/"); idx >= 0 {
		firstSeg = subPkg[:idx]
	}

	switch firstSeg {
	case "command", "query", "event":
		return KindPublic
	case "domain", "service", "repository":
		return KindInternal
	case "wire":
		return KindWire
	default:
		return KindOther
	}
}

func IsDDDPackage(pkgPath string) bool {
	return ParsePkgPath(pkgPath) != nil
}

func IsInternalPackage(pkgPath string) bool {
	info := ParsePkgPath(pkgPath)
	return info != nil && info.Kind == KindInternal
}

func SameDomain(pkgPath1, pkgPath2 string) bool {
	info1 := ParsePkgPath(pkgPath1)
	info2 := ParsePkgPath(pkgPath2)
	if info1 == nil || info2 == nil {
		return false
	}
	return info1.DDDPrefix == info2.DDDPrefix && info1.DomainName == info2.DomainName
}

func SameModule(pkgPath1, pkgPath2 string) bool {
	info1 := ParsePkgPath(pkgPath1)
	info2 := ParsePkgPath(pkgPath2)
	if info1 == nil || info2 == nil {
		return false
	}
	return info1.ModuleName == info2.ModuleName
}
