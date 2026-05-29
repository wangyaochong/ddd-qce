package core

import (
	"fmt"
	"runtime/debug"
	"time"
)

var Version = "(devel)"

func GetVersion() string {
	if Version != "(devel)" {
		return Version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "(devel)"
	}
	var revision, vcsTime string
	var modified bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.time":
			vcsTime = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if revision == "" {
		return "(devel)"
	}
	if len(revision) > 12 {
		revision = revision[:12]
	}
	suffix := ""
	if modified {
		suffix = "-dirty"
	}
	if vcsTime != "" {
		t, err := time.Parse(time.RFC3339, vcsTime)
		if err == nil {
			return fmt.Sprintf("v%s-%s%s", t.Format("20060102"), revision, suffix)
		}
	}
	return fmt.Sprintf("%s%s", revision, suffix)
}
