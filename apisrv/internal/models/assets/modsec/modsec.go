package modsec

import (
	"embed"
	"io/fs"
	"strings"
)

// Embed mod sec configuration template files
//
//go:embed profiles
var crsFS embed.FS

func CRSRuleSet(profileDir string) (map[string]string, error) {
	modsecConfig := map[string]string{}
	root := "profiles/" + profileDir
	err := fs.WalkDir(crsFS, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		// 2. Read the file content
		data, err := crsFS.ReadFile(path)
		if err != nil {
			return err
		}
		// remove "profiles/" + profileDir + "/" prefix from the path
		// expected path example 1: crs-setup.conf
		// expected path example 2: rules/REQUEST-943-APPLICATION-ATTACK-SESSION-FIXATION.conf
		modsecConfig[strings.Replace(path, root+"/", "", 1)] = string(data)
		return nil
	})

	if err != nil {
		return modsecConfig, err
	}

	return modsecConfig, nil
}
