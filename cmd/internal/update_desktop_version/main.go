package main

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"runtime"

	"github.com/sagernet/sing-box/cmd/internal/build_shared"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
)

var (
	flagRunInCI    bool
	flagRunNightly bool
)

type versionMetadata struct {
	Version   string `json:"version"`
	GoVersion string `json:"go_version"`
}

func init() {
	flag.BoolVar(&flagRunInCI, "ci", false, "Run in CI")
	flag.BoolVar(&flagRunNightly, "nightly", false, "Run nightly")
}

func main() {
	flag.Parse()
	newVersion := common.Must1(build_shared.ReadTag())
	desktopPath := "../sing-box-for-desktop"
	if flagRunInCI {
		desktopPath = "clients/desktop"
	}
	desktopPath = common.Must1(filepath.Abs(desktopPath))
	versionPath := filepath.Join(desktopPath, "version.json")
	versionFile := common.Must1(os.Open(versionPath))
	var metadata versionMetadata
	common.Must(json.NewDecoder(versionFile).Decode(&metadata))
	common.Must(versionFile.Close())
	newGoVersion := runtime.Version()
	versionUpdated := metadata.Version != newVersion
	goVersionUpdated := metadata.GoVersion != newGoVersion
	if !(versionUpdated || goVersionUpdated) {
		log.InfoEvent("cli.message", "version not changed")
		return
	}
	if versionUpdated {
		log.InfoEvent("cli.message", "updated version", log.String("old_version", metadata.Version), log.String("new_version", newVersion))
	}
	if goVersionUpdated {
		log.InfoEvent("cli.message", "updated Go version", log.String("old_version", metadata.GoVersion), log.String("new_version", newGoVersion))
	}
	if flagRunInCI && !flagRunNightly {
		log.FatalEvent("cli.error", "version changed, commit changes first")
	}
	metadata.Version = newVersion
	metadata.GoVersion = newGoVersion
	outputFile := common.Must1(os.Create(versionPath))
	encoder := json.NewEncoder(outputFile)
	encoder.SetIndent("", "  ")
	common.Must(encoder.Encode(metadata))
	common.Must(outputFile.Close())
}
