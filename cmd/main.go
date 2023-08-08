// go run cmd/*.go -f spec/TEST-DropCopy.cfg -a test-example-key -m drop_copy
// go run cmd/*.go -f spec/TEST-OrderEntry.cfg -a test-example-key -m order_entry
// go run cmd/*.go -f spec/TEST-OrderEntry.cfg -g spec/TEST-DropCopy.cfg -a test-example-key -m cancel_all
// go run cmd/*.go -f spec/TEST-OrderEntry.cfg -a test-example-key -m security_list -c securityListRequest
// go run cmd/*.go -f spec/TEST-OrderEntry.cfg -a test-example-key -m security_list -c securityDefinitionRequest
// Please create `<account_id>.api` with api_key and `<account_id>.pem` with private key

// If you don't want to generate Password on each Logon, you may generate a JWT expiring in the far future
// Duration should be in seconds, minutes or hours ('87600h' ~ 10 years)
// go run cmd/*.go -f spec/TEST-OrderEntry.cfg -a test-example-key -d '87600h' -m gen_password

package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/Power-Trade/fix-api-clients/pkg/fix"
)

var fixConfigPath = flag.String("f", "", "fix config file path")
var fixConfig2Path = flag.String("g", "", "fix config file path")
var fixMode = flag.String("m", "drop_copy", "mode: drop_copy, order_entry")
var apiKeyName = flag.String("a", "test-example-key", "api key")
var passwordDuration = flag.Duration("d", 10*365*24*time.Hour, "Duration of JWT (e.g. '87600h')")

func main() {
	var err error
	flag.Parse()

	switch *fixMode {
	case "drop_copy":
		err = fix.RunDropCopy(*fixConfigPath, *apiKeyName)
	case "order_entry":
		err = fix.RunOrderEntry(*fixConfigPath, *apiKeyName)
	case "security_list":
		err = fix.RunSecurityList(*fixConfigPath, *apiKeyName)
	case "cancel_all":
		err = fix.RunCancelAll(*fixConfigPath, *fixConfig2Path, *apiKeyName)
	case "gen_password":
		err = fix.RunGeneratePassword(*fixConfigPath, *apiKeyName, *passwordDuration)
	default:
		panic("unknown mode")
	}

	if err != nil {
		fmt.Printf("FixError: %v\n", err)
	}
}
