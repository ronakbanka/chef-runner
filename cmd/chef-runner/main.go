package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/mlafeldt/chef-runner.go/berkshelf"
	"github.com/mlafeldt/chef-runner.go/exec"
	"github.com/mlafeldt/chef-runner.go/metadata"
	"github.com/mlafeldt/chef-runner.go/rsync"
	"github.com/mlafeldt/chef-runner.go/util"
	"github.com/mlafeldt/chef-runner.go/vagrant"
)

const (
	CookbookPath    = "vendor/cookbooks"
	VagrantChefPath = "/tmp/vagrant-chef-1"
)

func cookbookFiles() ([]string, error) {
	filesGlob := []string{
		"README.*",
		"metadata.*",
		"attributes",
		"definitions",
		"files",
		"libraries",
		"providers",
		"recipes",
		"resources",
		"templates",
	}

	var files []string
	for _, glob := range filesGlob {
		matches, err := filepath.Glob(glob)
		if err != nil {
			return nil, err
		}
		files = append(files, matches...)
	}

	return files, nil
}

func installCookbooks(cookbookName, installDir string) error {
	if !util.FileExist(installDir) {
		return berkshelf.Install(installDir)
	}

	files, err := cookbookFiles()
	if err != nil {
		return err
	}
	opts := rsync.Options{
		Archive: true,
		Delete:  true,
		Verbose: true,
	}
	return rsync.Copy(files, path.Join(installDir, cookbookName), opts)
}

func openSSH(host, command string) error {
	return exec.RunCommand([]string{"ssh", host, "-c", command})
}

func provision(host, machine, format, logLevel, jsonFile string, runlist string) error {
	config_file := VagrantChefPath + "/solo.rb"
	json_file := VagrantChefPath + "/dna.json"
	cookbooks_path := "/vagrant/" + CookbookPath

	setup_dir := fmt.Sprintf("sudo mkdir -p %s", VagrantChefPath)
	setup_config := fmt.Sprintf("test -f %s || echo 'cookbook_path \"%s\"' | sudo tee %s >/dev/null", config_file, cookbooks_path, config_file)
	setup_json := fmt.Sprintf("test -f %s || echo '{}' | sudo tee %s >/dev/null", json_file, json_file)

	if jsonFile != "" {
		json_file = "/vagrant/" + jsonFile
	}

	run_chef_solo := fmt.Sprintf("sudo chef-solo --config=%s --json-attributes=%s --override-runlist=%s --format=%s --log_level=%s",
		config_file, json_file, runlist, format, logLevel)

	cmd := strings.Join([]string{setup_dir, setup_config, setup_json, run_chef_solo}, " && ")
	// fmt.Println(cmd)

	var err error
	if host != "" {
		err = openSSH(host, cmd)
	} else {
		err = vagrant.RunCommand(machine, cmd)
	}
	return err
}

func cookbookNameFromPath(cookbookPath string) string {
	base := path.Base(cookbookPath)
	if strings.HasPrefix(base, "chef-") {
		return strings.TrimPrefix(base, "chef-")
	}
	if strings.HasSuffix(base, "-cookbook") {
		return strings.TrimSuffix(base, "-cookbook")
	}
	return base
}

func buildRunList(cookbookName string, recipes []string) string {
	if len(recipes) == 0 {
		return cookbookName + "::default"
	}

	var runlist []string
	for _, r := range recipes {
		var recipeName string
		if strings.Contains(r, "::") {
			recipeName = r
		} else if path.Dir(r) == "recipes" && path.Ext(r) == ".rb" {
			recipeName = cookbookName + "::" + util.BaseName(r, ".rb")
		} else {
			recipeName = cookbookName + "::" + r
		}
		runlist = append(runlist, recipeName)
	}
	return strings.Join(runlist, ",")
}

func main() {
	log.SetFlags(0)

	var (
		host     = flag.String("H", "", "Set hostname for direct SSH access")
		machine  = flag.String("M", "", "Set name of Vagrant virtual machine")
		format   = flag.String("F", "null", "Set output format")
		logLevel = flag.String("l", "info", "Set log level")
		jsonFile = flag.String("j", "", "Load attributes from a JSON file")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: chef-runner [flags] [recipe ...]\n")
		flag.PrintDefaults()
		os.Exit(2)
	}
	flag.Parse()

	if *host != "" && *machine != "" {
		log.Fatal("error: -H and -M cannot be used together")
	}

	var cookbookName string
	if util.FileExist("metadata.rb") {
		metadata, err := metadata.ParseFile("metadata.rb")
		if err != nil {
			log.Fatal(err)
		}
		cookbookName = metadata.Name
	}
	if cookbookName == "" {
		cwd, err := os.Getwd()
		if err != nil {
			log.Fatal(err)
		}
		cookbookName = cookbookNameFromPath(cwd)
	}

	runlist := buildRunList(cookbookName, flag.Args())
	fmt.Println("Run List is", runlist)

	if err := installCookbooks(cookbookName, CookbookPath); err != nil {
		log.Fatal(err)
	}
	if err := provision(*host, *machine, *format, *logLevel, *jsonFile, runlist); err != nil {
		log.Fatal(err)
	}
}
