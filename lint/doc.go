// Package lint provides DDD static analysis rules for projects using the ddd-qce framework.
//
// # Available Analyzers
//
//   - dddcrossdomain: Checks that DDD domains do not import internal packages
//     from other domains. Only public packages (command, query, event) may be imported
//     across domain boundaries.
//
//   - dddpublicleak: Checks that public DDD packages (command, query, event) do
//     not reference internal domain types in exported struct fields or function signatures.
//
//   - dddimplimport: Checks that CQRS implementation packages (cqrs/impl/*) are only
//     imported from the wire layer, enforcing dependency inversion.
//
// # Directory Convention
//
// All domains reside under a ddd/ directory:
//
//	ddd/
//	├── order/
//	│   ├── command/     # public — cross-domain import allowed
//	│   ├── query/       # public
//	│   ├── event/       # public
//	│   ├── domain/      # internal — same-domain only
//	│   ├── service/     # internal
//	│   ├── repository/  # internal
//	│   └── wire/        # infrastructure — only place to import impl packages
//	└── inventory/
//	    └── ...
//
// # Integration with golangci-lint
//
// Add to .golangci.yml:
//
//	linters-settings:
//	  custom:
//	    dddcrossdomain:
//	      type: module
//	      description: "Check cross-domain internal package imports"
//	    dddpublicleak:
//	      type: module
//	      description: "Check domain type leaks in public packages"
//	    dddimplimport:
//	      type: module
//	      description: "Check CQRS impl package imports outside wire layer"
package lint
