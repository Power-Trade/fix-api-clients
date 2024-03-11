package fix

import (
	"flag"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Power-Trade/fix-api-clients/pkg/pt"
	"github.com/quickfixgo/enum"
	"github.com/quickfixgo/field"
	"github.com/quickfixgo/fix44/newordersingle"
	"github.com/quickfixgo/fix44/ordercancelrequest"
	"github.com/quickfixgo/quickfix"
	"github.com/quickfixgo/quickfix/config"
	"github.com/quickfixgo/tag"
	"github.com/shopspring/decimal"
)

var (
	uoCntCmd = flag.Int64("puo", 10000, "Perf: max count of unresponded orders")
)

type OrderState int

type OrderInfo struct {
	State  OrderState
	SentAt time.Time
}

const (
	OrderState_UNKNOWN   OrderState = 0
	OrderState_SENT      OrderState = 1
	OrderState_CREATED   OrderState = 2
	OrderState_CANCELLED OrderState = 3
)

var (
	cntSent      atomic.Int64
	cntCreated   atomic.Int64
	cntClosed    atomic.Int64
	cntBsnReject atomic.Int64
	delayCreated time.Duration

	activeOrders   = make(map[string]*OrderInfo) // ClOrdId -> OrderState
	activeOrdersMu sync.Mutex
)

type PerfTradeClient struct {
	*TradeClient
}

func (e PerfTradeClient) ToApp(msg *quickfix.Message, sessionID quickfix.SessionID) (err error) {
	return
}

func (e PerfTradeClient) FromApp(msg *quickfix.Message, sessionID quickfix.SessionID) (err quickfix.MessageRejectError) {
	now := time.Now()

	typeField := field.MsgTypeField{}
	err = msg.Header.Get(&typeField)
	if err != nil {
		return
	}

	switch typeField.Value() {
	case enum.MsgType_EXECUTION_REPORT:
		clOrdIDStr := ""
		ordStatus := field.OrdStatusField{}
		err = msg.Body.Get(&ordStatus)
		if err != nil {
			return
		}

		isOrderActive := true
		if ordStatus.Value() == enum.OrdStatus_CANCELED {
			isOrderActive = false

			clOrdID := field.OrigClOrdIDField{}
			err = msg.Body.Get(&clOrdID)
			if err != nil {
				return
			}
			clOrdIDStr = clOrdID.Value()
		} else {
			leavesQtyStr, _ := msg.Body.GetString(tag.LeavesQty)
			leavesQty, _ := decimal.NewFromString(leavesQtyStr)
			if leavesQty.IsZero() {
				isOrderActive = false
			}

			clOrdID := field.ClOrdIDField{}
			err = msg.Body.Get(&clOrdID)
			if err != nil {
				return
			}
			clOrdIDStr = clOrdID.Value()
		}

		activeOrdersMu.Lock()
		defer activeOrdersMu.Unlock()

		orderInfo := activeOrders[clOrdIDStr]
		if orderInfo == nil {
			orderInfo = &OrderInfo{
				State:  OrderState_UNKNOWN,
				SentAt: time.Time{},
			}
		}

		switch orderInfo.State {
		case OrderState_SENT:
			cntCreated.Add(1)
			delay := now.Sub(orderInfo.SentAt)
			// fmt.Printf("Order[%s]: %s -> %s\n", clOrdIDStr, orderInfo.SentAt.Format(time.RFC3339Nano), delay.String())
			delayCreated += delay
			if !isOrderActive {
				cntClosed.Add(1)
				orderInfo.State = OrderState_CANCELLED
			} else {
				orderInfo.State = OrderState_CREATED
			}

		case OrderState_CREATED:
			if !isOrderActive {
				cntClosed.Add(1)
				orderInfo.State = OrderState_CANCELLED
			}
		case OrderState_CANCELLED:
		default:
		}

	case enum.MsgType_BUSINESS_MESSAGE_REJECT:
		cntBsnReject.Add(1)

	default:
	}

	return
}

func PrintStat() {
	timeSt := time.Now().UTC()
	for {
		time.Sleep(1 * time.Second)

		now := time.Now().UTC()
		dur := now.Sub(timeSt)

		ordPerSecond := int64(time.Duration(cntCreated.Load()+cntClosed.Load()) * time.Second / dur)
		avgDelay := time.Duration(-1)
		if cntCreated.Load() > 0 {
			avgDelay = (delayCreated / time.Duration(cntCreated.Load()))
		}
		fmt.Printf("Stats: %s %d %d %d %d  Perf=%v orders/sec Delay=%v\n",
			dur.String(), cntSent.Load(), cntCreated.Load(), cntClosed.Load(), cntBsnReject.Load(),
			ordPerSecond, avgDelay,
		)
	}
}

func RunOrderEntryPerf(cfgFileName string, apiKeyName string) error {
	tapp, err := NewTradeClient(cfgFileName, apiKeyName)
	if err != nil {
		return err
	}

	app := PerfTradeClient{tapp}

	err = StartConnection(app, app.Settings)
	if err != nil {
		return err
	}
	targetCompID, _ := app.Settings.GlobalSettings().Setting(config.TargetCompID)
	sessionID := quickfix.SessionID{
		BeginString:  string("FIX.4.4"),
		TargetCompID: string(targetCompID),
		SenderCompID: string(app.SenderCompID),
	}

	app.WaitConnect()

	go PrintStat()

	for {
		// time.Sleep(time.Second)

		if cntSent.Load()-cntCreated.Load()-cntBsnReject.Load() >= *uoCntCmd {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		clOrdIDStr := fmt.Sprint(pt.DefaultTokenGenerator.Next())

		{
			order := newordersingle.New(
				field.NewClOrdID(clOrdIDStr), // ToDo: FIX server should allow a non-duplicate char[19], not only increasing int56
				field.NewSide(enum.Side_BUY),
				field.NewTransactTimeWithPrecision(time.Now(), quickfix.Nanos),
				field.NewOrdType(enum.OrdType_LIMIT),
			)
			//order.Set(field.NewSendingTime(time.Now()))
			order.Set(field.NewSymbol("BTC-USD"))
			order.Set(field.NewOrderQty(decimal.NewFromFloat(0.01), 2))

			order.Set(field.NewPrice(decimal.NewFromFloat(1), 0))

			order.Set(field.NewTimeInForce(enum.TimeInForce_GOOD_TILL_CANCEL))
			//order.Set(field.NewExpireTimeWithPrecision(time.Now().AddDate(0, 0, 1), quickfix.Nanos))

			msg := order.ToMessage()
			msg.Header.Set(field.NewSenderCompID(app.SenderCompID))
			msg.Header.Set(field.NewTargetCompID(targetCompID))

			orderInfo := &OrderInfo{
				State:  OrderState_SENT,
				SentAt: time.Now(),
			}
			activeOrdersMu.Lock()
			activeOrders[clOrdIDStr] = orderInfo
			activeOrdersMu.Unlock()

			//fmt.Printf("Sending[%s]: %s\n", clOrdID, msg.String())

			err := quickfix.SendToTarget(msg, sessionID)

			if err != nil {
				return err
			}
		}

		{
			cancelClOrdId := fmt.Sprint(pt.DefaultTokenGenerator.Next())

			order := ordercancelrequest.New(
				field.NewOrigClOrdID(clOrdIDStr),
				field.NewClOrdID(cancelClOrdId),
				field.NewSide(enum.Side_BUY),
				field.NewTransactTime(time.Now()),
			)

			msg := order.ToMessage()
			msg.Header.Set(field.NewSenderCompID(app.SenderCompID))
			msg.Header.Set(field.NewTargetCompID(targetCompID))

			// fmt.Printf("Sending: %s\n", msg.String())

			err := quickfix.SendToTarget(msg, sessionID)

			if err != nil {
				return err
			}
		}

		cntSent.Add(1)
	}
}
