package util

import (
	"io/ioutil"
	"os"
	"path"
	"strings"
)

func FileExist(name string) bool {
	_, err := os.Stat(name)
	return err == nil
}

func BaseName(s, suffix string) string {
	base := path.Base(s)
	if suffix != "" {
		base = strings.TrimSuffix(base, suffix)
	}
	return base
}

func TempDir() (string, error) {
	return ioutil.TempDir("", "chef-runner-")
}
