package buildinfo

import "runtime/debug"

var Version = "dev"

type Info struct {
	Version string
	Commit  string
	BuiltAt string
}

func Get() Info {
	info := Info{Version: Version}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				info.Commit = s.Value
			case "vcs.time":
				info.BuiltAt = s.Value
			}
		}
	}
	return info
}
