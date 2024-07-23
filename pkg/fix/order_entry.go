package fix

import (
	"flag"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/Power-Trade/fix-api-clients/pkg/pt"
	"github.com/quickfixgo/enum"
	"github.com/quickfixgo/field"
	"github.com/quickfixgo/fix44/heartbeat"
	"github.com/quickfixgo/fix44/newordermultileg"
	"github.com/quickfixgo/fix44/newordersingle"
	"github.com/quickfixgo/fix44/ordercancelrequest"
	"github.com/quickfixgo/quickfix"
	"github.com/quickfixgo/quickfix/config"
	"github.com/shopspring/decimal"
)

type FIXExampleAction = func() *quickfix.Message

var (
	lastMessageClOrdId = ""

	possibleActionsOE = []FIXExampleAction{
		addOrder,
		addOrderMatch,
		cancelOrder, // Cancel existing order
		cancelOrder, // Cancel unknown order
		addOrderMultiLeg,
		addOrderExecInst,
		sendHB,
	}

	actionsCmd = flag.String("c", "", "Action list") // addOrder,cancelOrder,addOrderTrigger
)

func addOrder() *quickfix.Message {
	clOrdId := fmt.Sprint(pt.DefaultTokenGenerator.Next())

	lastMessageClOrdId = clOrdId // Store for cancel

	order := newordersingle.New(
		field.NewClOrdID(clOrdId), // ToDo: FIX server should allow a non-duplicate char[19], not only increasing int56
		field.NewSide(enum.Side_BUY),
		field.NewTransactTime(time.Now()),
		field.NewOrdType(enum.OrdType_LIMIT),
	)
	order.Set(field.NewSymbol("BTC-USD"))
	order.Set(field.NewOrderQty(decimal.NewFromFloat(0.15), 4))

	order.Set(field.NewPrice(decimal.NewFromInt(22150), 2))

	order.Set(field.NewTimeInForce(enum.TimeInForce_GOOD_TILL_DATE))
	order.Set(field.NewExpireTime(time.Now().AddDate(0, 0, 1)))

	// Optional SecondaryClOrdID: up to 17 ASCII symbols
	order.Set(field.NewSecondaryClOrdID(time.Now().Format("15:04:05.999999")))

	return order.ToMessage()
}

func addOrderMatch() *quickfix.Message {
	clOrdId := fmt.Sprint(pt.DefaultTokenGenerator.Next())

	order := newordersingle.New(
		field.NewClOrdID(clOrdId),
		field.NewSide(enum.Side_SELL),
		field.NewTransactTime(time.Now()),
		field.NewOrdType(enum.OrdType_MARKET),
	)
	order.Set(field.NewSymbol("BTC-USD"))
	order.Set(field.NewOrderQty(decimal.NewFromFloat(0.08), 4))

	order.Set(field.NewTimeInForce(enum.TimeInForce_IMMEDIATE_OR_CANCEL))

	return order.ToMessage()
}

func cancelOrder() *quickfix.Message {
	if lastMessageClOrdId == "" {
		lastMessageClOrdId = "11" // Non-existing order_token
	}

	cancel := ordercancelrequest.New(
		field.NewOrigClOrdID(lastMessageClOrdId),
		field.NewClOrdID(fmt.Sprint(pt.DefaultTokenGenerator.Next())),
		field.NewSide(enum.Side_BUY),
		field.NewTransactTime(time.Now()),
	)

	lastMessageClOrdId = ""

	return cancel.ToMessage()
}

func addOrderMultiLeg() *quickfix.Message {
	clOrdId := fmt.Sprint(pt.DefaultTokenGenerator.Next())

	lastMessageClOrdId = clOrdId // Store for cancel

	order := newordermultileg.New(
		field.NewClOrdID(clOrdId), // ToDo: FIX server should allow a non-duplicate char[19], not only increasing int56
		field.NewSide(enum.Side_BUY),
		field.NewTransactTime(time.Now()),
		field.NewOrdType(enum.OrdType_LIMIT),
	)

	// !! ExecutionReport may have Legs sorted in different way (and reverted Side to have 1st Leg's Ratio > 0)
	Legs := newordermultileg.NewNoLegsRepeatingGroup()
	Legs.Add()
	Legs.Get(0).Set(field.NewLegSymbol("BTC-USD"))
	Legs.Get(0).Set(field.NewLegRatioQty(decimal.NewFromInt(1), 0))
	Legs.Add()
	Legs.Get(1).Set(field.NewLegSymbol("ETH-USD"))
	Legs.Get(1).Set(field.NewLegRatioQty(decimal.NewFromInt(-1), 0))
	order.SetGroup(Legs)

	order.SetSymbolSfx("none") // `market_id=none`, i.e. it is an RFQ order

	order.Set(field.NewOrderQty(decimal.NewFromFloat(0.1), 4))
	order.Set(field.NewPrice(decimal.NewFromInt(16), 2))
	order.Set(field.NewTimeInForce(enum.TimeInForce_GOOD_TILL_DATE))
	order.Set(field.NewExpireTime(time.Now().AddDate(0, 0, 1)))

	return order.ToMessage()
}

func addOrderExecInst() *quickfix.Message {
	clOrdId := fmt.Sprint(pt.DefaultTokenGenerator.Next())

	lastMessageClOrdId = clOrdId // Store for cancel

	order := newordersingle.New(
		field.NewClOrdID(clOrdId), // ToDo: FIX server should allow a non-duplicate char[19], not only increasing int56
		field.NewSide(enum.Side_SELL),
		field.NewTransactTime(time.Now()),
		field.NewOrdType(enum.OrdType_LIMIT),
	)
	order.Set(field.NewSymbol("PTF-USD"))
	order.Set(field.NewOrderQty(decimal.NewFromFloat(2), 4))
	order.Set(field.NewPrice(decimal.NewFromFloat(0.3), 4))

	order.Set(field.NewTimeInForce(enum.TimeInForce_GOOD_TILL_DATE))
	order.Set(field.NewExpireTime(time.Now().AddDate(0, 0, 1)))

	order.Set(field.NewExecInst(enum.ExecInst_PARTICIPANT_DONT_INITIATE))

	return order.ToMessage()
}

func sendHB() *quickfix.Message {
	hrtbt := heartbeat.New()
	return hrtbt.ToMessage()
}

func getActions(possibleActions []FIXExampleAction) []FIXExampleAction {
	actionCmds := strings.Split(*actionsCmd, ",")
	if len(actionCmds) == 0 || actionCmds[0] == "" {
		return possibleActions
	}

	possibleActionMap := make(map[string]FIXExampleAction)
	for _, action := range possibleActions {
		fullname := runtime.FuncForPC(reflect.ValueOf(action).Pointer()).Name()
		fullnames := strings.Split(fullname, ".")
		name := fullnames[len(fullnames)-1]
		possibleActionMap[name] = action
	}

	actions := make([]FIXExampleAction, 0)
	for _, actionCmd := range actionCmds {
		action := possibleActionMap[actionCmd]
		if action == nil {
			panic(fmt.Sprintf("Unknown action: '%s'", actionCmd))
		}
		actions = append(actions, action)
	}
	return actions
}

func RunOrderEntry(cfgFileName string, apiKeyName string) error {
	app, err := NewTradeClient(cfgFileName, apiKeyName)
	if err != nil {
		return err
	}

	err = StartConnection(app, app.Settings)
	if err != nil {
		return err
	}
	targetCompID, _ := app.Settings.GlobalSettings().Setting(config.TargetCompID)

	actions := getActions(possibleActionsOE)
	for {
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
	}
}

func RunOrderEntryManual(cfgFileName string, apiKeyName string) error {
	app, err := NewTradeClient(cfgFileName, apiKeyName)
	if err != nil {
		return err
	}

	err = StartConnection(app, app.Settings)
	if err != nil {
		return err
	}
	targetCompID, _ := app.Settings.GlobalSettings().Setting(config.TargetCompID)

	for {
		time.Sleep(time.Second)

		//clOrdId := fmt.Sprint(pt.DefaultTokenGenerator.Next())
		clOrdId := "123321"

		lastMessageClOrdId = clOrdId // Store for cancel

		order := newordersingle.New(
			field.NewClOrdID(clOrdId), // ToDo: FIX server should allow a non-duplicate char[19], not only increasing int56
			field.NewSide(enum.Side_BUY),
			field.NewTransactTimeWithPrecision(time.Now(), quickfix.Nanos),
			field.NewOrdType(enum.OrdType_LIMIT),
		)
		//order.Set(field.NewSendingTime(time.Now()))
		order.Set(field.NewSymbol("BTC-USD-PERPETUAL"))
		order.Set(field.NewOrderQty(decimal.NewFromFloat(0.01), 2))

		order.Set(field.NewPrice(decimal.NewFromFloat(1), 0))

		order.Set(field.NewTimeInForce(enum.TimeInForce_GOOD_TILL_CANCEL))
		//order.Set(field.NewExpireTimeWithPrecision(time.Now().AddDate(0, 0, 1), quickfix.Nanos))

		msg := order.ToMessage()

		msg.Header.Set(field.NewSenderCompID(app.SenderCompID))
		msg.Header.Set(field.NewTargetCompID(targetCompID))

		fmt.Printf("Sending: %s\n", msg.String())

		err := quickfix.Send(msg)

		if err != nil {
			return err
		}
	}
}
