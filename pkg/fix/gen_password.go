package fix

import (
	"fmt"
	"time"

	"github.com/Power-Trade/fix-api-clients/pkg/pt"
	"github.com/quickfixgo/quickfix/config"
)

func RunGeneratePassword(cfgFilename string, apiKeyName string, dur time.Duration) error {
	apiKey, privateKey, err := pt.ReadKeys(apiKeyName)
	if err != nil {
		return err
	}

	settings, err := ReadConfig(cfgFilename, apiKey)
	if err != nil {
		return err
	}

	host, _ := settings.GlobalSettings().Setting(config.SocketConnectHost)

	password, err := pt.GeneratePassword(
		apiKey,
		privateKey,
		host,
		"ES256",
		"api",
		dur,
	)

	fmt.Printf("JWT: %s\n", password)

	return nil
}
