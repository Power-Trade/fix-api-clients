package fix

import (
	"fmt"
	"time"

	"github.com/Power-Trade/fix-api-clients/pkg/pt"
	"github.com/quickfixgo/enum"
	"github.com/quickfixgo/field"
	"github.com/quickfixgo/fix44/securitydefinitionrequest"
	"github.com/quickfixgo/fix44/securitylistrequest"
	"github.com/quickfixgo/quickfix"
	"github.com/quickfixgo/quickfix/config"
)

var (
	possibleActionsSL = []FIXExampleAction{
		securityListRequest,
		securityDefinitionRequest,
	}
)

func securityListRequest() *quickfix.Message {
	clOrdId := fmt.Sprint(pt.DefaultTokenGenerator.Next())

	order := securitylistrequest.New(
		field.NewSecurityReqID(clOrdId),
		field.NewSecurityListRequestType(enum.SecurityListRequestType_ALL_SECURITIES),
	)
	// order.Set(field.NewSymbol("BTC-USD"))

	return order.ToMessage()
}

func securityDefinitionRequest() *quickfix.Message {
	clOrdId := fmt.Sprint(pt.DefaultTokenGenerator.Next())

	order := securitydefinitionrequest.New(
		field.NewSecurityReqID(clOrdId),
		field.NewSecurityRequestType(enum.SecurityRequestType_REQUEST_LIST_SECURITIES),
	)
	// order.Set(field.NewSymbol("BTC-USD"))

	return order.ToMessage()
}

func RunSecurityList(cfgFileName string, apiKeyName string) error {
	app, err := NewTradeClient(cfgFileName, apiKeyName)
	if err != nil {
		return err
	}

	err = StartConnection(app, app.Settings)
	if err != nil {
		return err
	}
	targetCompID, _ := app.Settings.GlobalSettings().Setting(config.TargetCompID)

	actions := getActions(possibleActionsSL)
	for _, action := range actions {
		time.Sleep(time.Second)

		msg := action()
		msg.Header.Set(field.NewSenderCompID(app.SenderCompID))
		msg.Header.Set(field.NewTargetCompID(targetCompID))

		fmt.Printf("Sending: %s\n", msg.String())

		err := quickfix.Send(msg)

		if err != nil {
			return err
		}
	}

	for {
		time.Sleep(time.Second)
	}
}
