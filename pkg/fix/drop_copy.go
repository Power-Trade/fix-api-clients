package fix

import (
	"time"
)

func RunDropCopy(cfgFileName string, apiKeyName string) error {
	app, err := NewTradeClient(cfgFileName, apiKeyName)
	if err != nil {
		return err
	}

	StartConnection(app, app.Settings)

	for {
		time.Sleep(time.Second)
	}
}
