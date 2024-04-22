package fix

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/Power-Trade/fix-api-clients/pkg/pt"
	"github.com/quickfixgo/enum"
	"github.com/quickfixgo/field"
	"github.com/quickfixgo/quickfix"
	"github.com/quickfixgo/quickfix/config"
)

var (
	loggerCmd = flag.String("l", "file", "Log") // file/no
)

// TradeClient implements the quickfix.Application interface
type TradeClient struct {
	SenderCompID string
	PrivateKey   []byte
	Settings     *quickfix.Settings
	connected    bool
	connectCond  *sync.Cond
}

type ApplicationWithWait interface {
	quickfix.Application
	WaitConnect() bool
}

func NewTradeClient(cfgFilename string, keyFilename string) (*TradeClient, error) {
	apiKey, privateKey, err := pt.ReadKeys(keyFilename)
	if err != nil {
		return nil, err
	}

	settings, err := ReadConfig(cfgFilename, apiKey)
	if err != nil {
		return nil, err
	}

	app := &TradeClient{
		SenderCompID: apiKey,
		PrivateKey:   privateKey,
		Settings:     settings,
		connected:    false,
		connectCond:  sync.NewCond(&sync.Mutex{}),
	}
	return app, nil
}

func ReadConfig(cfgFilename string, apiKey string) (*quickfix.Settings, error) {
	cfg, err := os.Open(cfgFilename)
	if err != nil {
		return nil, fmt.Errorf("open '%v': %v", cfgFilename, err)
	}
	defer cfg.Close()

	stringData, readErr := io.ReadAll(cfg)
	if readErr != nil {
		return nil, fmt.Errorf("error reading cfg: %s,", readErr)
	}

	// Quickfix doesn't allow settings without at least 1 SESSION
	stringData = append(stringData, []byte(`
[SESSION]
BeginString=FIX.4.4
SenderCompID=`+apiKey+`
		`)...)

	settings, err := quickfix.ParseSettings(bytes.NewReader(stringData))
	if err != nil {
		return nil, fmt.Errorf("error reading cfg: %s,\n%s", err, stringData)
	}

	return settings, nil
}

func StartConnection(app ApplicationWithWait, settings *quickfix.Settings) error {
	var logFactory quickfix.LogFactory

	switch *loggerCmd {
	case "file":
		logFactory = NewBeautyLogFactory(quickfix.NewScreenLogFactory())
	case "no":
		logFactory = quickfix.NewNullLogFactory()
	default:
		panic(fmt.Sprintf("unknown log: %s", *loggerCmd))
	}

	initiator, err := quickfix.NewInitiator(app, quickfix.NewMemoryStoreFactory(), settings, logFactory)
	if err != nil {
		return fmt.Errorf("unable to create Initiator: %s", err)
	}

	err = initiator.Start()
	if err != nil {
		return fmt.Errorf("unable to start Initiator: %s", err)
	}

	// Wait for connection being established - otherwise QuickFIX will drop our requests
	isConnected := app.WaitConnect()
	if !isConnected {
		return errors.New("not connected")
	}

	return nil
}

// OnCreate implemented as part of Application interface
func (e *TradeClient) OnCreate(sessionID quickfix.SessionID) {}

// OnLogon implemented as part of Application interface
func (e *TradeClient) OnLogon(sessionID quickfix.SessionID) {}

// OnLogout implemented as part of Application interface
func (e *TradeClient) OnLogout(sessionID quickfix.SessionID) {}

// FromAdmin implemented as part of Application interface
func (e *TradeClient) FromAdmin(msg *quickfix.Message, sessionID quickfix.SessionID) (reject quickfix.MessageRejectError) {
	msgTypeStr, _ := msg.MsgType()
	msgType := enum.MsgType(msgTypeStr)
	if msgType == enum.MsgType_LOGON {
		e.connectCond.L.Lock()
		e.connected = true
		e.connectCond.L.Unlock()
		e.connectCond.Broadcast()
	} else if msgType == enum.MsgType_LOGOUT {
		e.connectCond.L.Lock()
		e.connected = false
		e.connectCond.L.Unlock()
		e.connectCond.Broadcast()
	}
	return nil
}

// ToAdmin implemented as part of Application interface
func (e *TradeClient) ToAdmin(msg *quickfix.Message, sessionID quickfix.SessionID) {
	msgTypeStr, _ := msg.MsgType()
	msgType := enum.MsgType(msgTypeStr)
	if msgType == enum.MsgType_LOGON {
		server, err := e.Settings.SessionSettings()[sessionID].Setting(config.SocketConnectHost)
		if err != nil {
			panic(err)
		}
		password, err := pt.GeneratePassword(e.SenderCompID, e.PrivateKey, server, "ES256", "api", 15*time.Second)
		if err != nil {
			panic(err)
		}
		msg.Body.Set(field.NewPassword(password))
		msg.Body.Set(field.NewResetSeqNumFlag(true))
	}
	fmt.Printf("\n[TO ADMIN]:\n")
}

// ToApp implemented as part of Application interface
func (e *TradeClient) ToApp(msg *quickfix.Message, sessionID quickfix.SessionID) (err error) {
	fmt.Printf("\n[TO APP]:\n")
	return
}

// FromApp implemented as part of Application interface. This is the callback for all Application level messages from the counter party.
func (e *TradeClient) FromApp(msg *quickfix.Message, sessionID quickfix.SessionID) (reject quickfix.MessageRejectError) {
	fmt.Printf("[FROM APP]\n\n")
	return
}

func (e *TradeClient) WaitConnect() bool {
	e.connectCond.L.Lock()
	defer e.connectCond.L.Unlock()
	for !e.connected {
		e.connectCond.Wait()
	}
	return true
}
