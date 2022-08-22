package fix

import (
	"fmt"
	"time"

	"github.com/Power-Trade/fix-api-clients/pkg/pt"
	"github.com/quickfixgo/enum"
	"github.com/quickfixgo/field"
	"github.com/quickfixgo/fix44/ordercancelrequest"
	"github.com/quickfixgo/quickfix"
	"github.com/quickfixgo/quickfix/config"
	"github.com/quickfixgo/tag"
)

type Canceller struct {
	*TradeClient
	senderOE string
	targetOE string
}

func (e *Canceller) FromApp(msg *quickfix.Message, sessionID quickfix.SessionID) (reject quickfix.MessageRejectError) {
	msgTypeStr, _ := msg.MsgType()
	msgType := enum.MsgType(msgTypeStr)
	if msgType == enum.MsgType_EXECUTION_REPORT {

		ordStatus := field.NewOrdStatus(enum.OrdStatus_NEW)
		msg.Body.Get(&ordStatus)

		// Nothing to do if the order is already cancelled
		if ordStatus.Value() == enum.OrdStatus_CANCELED {
			return
		}

		// Send OrderCancelRequest with OrderID
		orderID, _ := msg.Body.GetString(tag.OrderID)
		cancel := ordercancelrequest.New(
			field.NewOrigClOrdID("NONE"),
			field.NewClOrdID(fmt.Sprint(pt.DefaultTokenGenerator.Next())),
			field.NewSide(enum.Side_BUY),
			field.NewTransactTime(time.Now()),
		)
		cancel.Set(field.NewOrderID(orderID))

		msg := cancel.ToMessage()
		msg.Header.Set(field.NewSenderCompID(e.senderOE))
		msg.Header.Set(field.NewTargetCompID(e.targetOE))

		fmt.Printf("Sending: %s\n", msg.String())

		quickfix.Send(msg)
	}
	return
}

func RunCancelAll(cfgFilenameOE string, cfgFilenameDC string, apiKeyName string) error {

	appOE, err := NewTradeClient(cfgFilenameOE, apiKeyName)
	if err != nil {
		return err
	}
	targetCompID, _ := appOE.Settings.GlobalSettings().Setting(config.TargetCompID)
	StartConnection(appOE, appOE.Settings)

	appDC, err := NewTradeClient(cfgFilenameDC, apiKeyName)
	if err != nil {
		return err
	}
	appDCCanceller := &Canceller{
		TradeClient: appDC,
		senderOE:    appOE.SenderCompID,
		targetOE:    targetCompID,
	}
	StartConnection(appDCCanceller, appDCCanceller.Settings)

	for {
		time.Sleep(time.Second)
	}
}
