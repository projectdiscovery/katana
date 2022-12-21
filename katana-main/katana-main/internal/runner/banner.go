package runner

import (
	"fmt"

	"github.com/projectdiscovery/gologger"
)

var banner = fmt.Sprintf(`
   __        __                
  / /_____ _/ /____ ____  ___ _
 /  '_/ _  / __/ _  / _ \/ _  /
/_/\_\\_,_/\__/\_,_/_//_/\_,_/ %s							 
`, version)

var version = "v0.0.2"

// showBanner is used to show the banner to the user
func showBanner() {
	gologger.Print().Msgf("%s\n", banner)
	gologger.Print().Msgf("\t\tprojectdiscovery.io\n\n")

	gologger.Print().Label("WRN").Msgf("Use with caution. You are responsible for your actions.\n")
	gologger.Print().Label("WRN").Msgf("Developers assume no liability and are not responsible for any misuse or damage.\n")
}
