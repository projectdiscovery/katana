package main

import (
	"github.com/pkg/errors"
	"github.com/projectdiscovery/goflags"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/types"
)

var (
	cfgFile string
	options = &types.Options{}
)

func main() {
	if err := readFlags(); err != nil {
		gologger.Fatal().Msgf("Could not read flags: %s\n", err)
	}

}

func readFlags() error {
	flagSet := goflags.NewFlagSet()
	flagSet.SetDescription(`Katana is a fast crawler focused on execution in automation
pipelines offering both headless and non-headless crawling.`)

	createGroup(flagSet, "configs", "Configurations",
		flagSet.StringVar(&cfgFile, "config", "", "path to the nuclei configuration file"),
	)

	if err := flagSet.Parse(); err != nil {
		return errors.Wrap(err, "could not parse flags")
	}

	if cfgFile != "" {
		if err := flagSet.MergeConfigFile(cfgFile); err != nil {
			return errors.Wrap(err, "could not read config file")
		}
	}
	return nil
}

func createGroup(flagSet *goflags.FlagSet, groupName, description string, flags ...*goflags.FlagData) {
	flagSet.SetGroup(groupName, description)
	for _, currentFlag := range flags {
		currentFlag.Group(groupName)
	}
}
