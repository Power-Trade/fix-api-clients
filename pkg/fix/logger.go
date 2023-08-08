package fix

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/quickfixgo/field"
	"github.com/quickfixgo/quickfix"
	"github.com/quickfixgo/quickfix/datadictionary"
)

type BeautyLogFactory struct {
	parent quickfix.LogFactory
}

const (
	fixMessagePart_Header  = 0
	fixMessagePart_Body    = 1
	fixMessagePart_Trailer = 2
	fixMessagePart_Group   = 3

	printPrefix         = 32
	printPrefixInc      = 4
	printPrefixIndexInc = 2
	printArrayRepeat    = 8

	FIX_XML_PATH = "spec/FIX44-PT.xml"
)

func (b BeautyLogFactory) Create() (quickfix.Log, error) {
	plog, _ := b.parent.Create()
	dictionary, err := datadictionary.Parse(FIX_XML_PATH)
	if err != nil {
		fmt.Printf("datadictionary.Parse() -> ERROR: %v", err)
		return nil, err
	}
	log := BeautyLog{plog, "GLOBAL", dictionary}
	return log, nil
}

func (b BeautyLogFactory) CreateSessionLog(sessionID quickfix.SessionID) (quickfix.Log, error) {
	plog, _ := b.parent.CreateSessionLog(sessionID)
	dictionary, err := datadictionary.Parse(FIX_XML_PATH)
	if err != nil {
		fmt.Printf("datadictionary.Parse() -> ERROR: %v", err)
		return nil, err
	}
	log := BeautyLog{plog, sessionID.String(), dictionary}
	return log, nil
}

// NewScreenLogFactory creates an instance of LogFactory that writes messages and events to stdout.
func NewBeautyLogFactory(parent quickfix.LogFactory) quickfix.LogFactory {
	return BeautyLogFactory{parent}
}

type BeautyLog struct {
	quickfix.Log
	sessionName string
	dictionary  *datadictionary.DataDictionary
}

func (b BeautyLog) BeautifyFIX(raw []byte) []byte {

	msg := quickfix.NewMessage()

	err := quickfix.ParseMessage(msg, bytes.NewBuffer(raw))
	if err != nil {
		return []byte(fmt.Sprintf("Error: %v\n%s", err, raw))
	}

	var msgType field.MsgTypeField
	msg.Header.Get(&msgType)

	MessageDesc := b.dictionary.Messages[string(msgType.Value())]
	humanReadableFIX := b.BeautifyFIXString(string(raw))

	// Ideal result using Java: https://stackoverflow.com/questions/6453879/how-to-log-quickfix-message-in-human-readable-format
	return []byte(fmt.Sprintf("\nORIG:\n%s\n\nHEADER:\n%s\nBODY:\n%s\nTRAILER:\n%s\n",
		humanReadableFIX,
		b.BeautifyFieldMap(msg.Header.FieldMap, NewFieldDefs(b.dictionary.Header.Parts, b.dictionary.Header.Fields), fixMessagePart_Header, printPrefix, -1),
		b.BeautifyFieldMap(msg.Body.FieldMap, NewFieldDefs(MessageDesc.Parts, MessageDesc.Fields), fixMessagePart_Body, printPrefix, -1),
		b.BeautifyFieldMap(msg.Trailer.FieldMap, NewFieldDefs(b.dictionary.Trailer.Parts, b.dictionary.Trailer.Fields), fixMessagePart_Trailer, printPrefix, -1),
	))
}

func (b BeautyLog) BeautifyFIXString(event string) string {
	return strings.ReplaceAll(event, "\x01", "|")
}

type FieldDefs []*datadictionary.FieldDef

func (t FieldDefs) Find(tag quickfix.Tag) *datadictionary.FieldDef {
	for _, fieldDef := range t {
		if fieldDef.Tag() == int(tag) {
			return fieldDef
		}
	}
	return nil
}

func NewFieldDefs(Parts []datadictionary.MessagePart, MapFields map[int]*datadictionary.FieldDef) FieldDefs {
	values := make([]*datadictionary.FieldDef, 0)
	for _, part := range Parts {
		// fmt.Printf("Looking for %s\n", part.Name())
		switch rpart := part.(type) {
		case datadictionary.Component:
			{ // e.g. SecListGrp
				for _, v := range rpart.Fields() {
					values = append(values, v)
				}
			}
		default:
			{
				for _, v := range MapFields {
					if v.Name() == part.Name() {
						values = append(values, v)
						break
					}
				}
			}
		}
	}
	return values
}

func NewFieldDefsFromArr(MapFields []*datadictionary.FieldDef) FieldDefs {
	return MapFields
}

func (b BeautyLog) BeautifyField(tag quickfix.Tag, name string, value quickfix.FIXBytes, desc string, prefix int, index int) string {

	res := fmt.Sprintf("\t[%4d]", tag)
	if index >= 0 {
		// 1st field of an array
		idxOffset := (prefix - printPrefix) * printPrefixIndexInc / printPrefixInc
		idxStr := strings.Repeat(" ", idxOffset) + fmt.Sprintf("#%d", index)

		res += fmt.Sprintf(
			"%s    %*s",
			idxStr,
			(prefix - len(idxStr)),
			name,
		)
	} else {
		res += fmt.Sprintf(
			"    %*s",
			prefix,
			name,
		)
	}
	res += fmt.Sprintf(": %s [%s]", value, desc)

	res += "\n"
	return res
}

func (b BeautyLog) BeautifyFieldMap(fm quickfix.FieldMap, fieldDefs FieldDefs, messagePart int, prefix int, index int) string {

	var res string
	if messagePart != fixMessagePart_Group {
		res = "--------------------------------------------------------------------\n"
	}

	for _, gFieldDef := range fieldDefs {
		var value quickfix.FIXBytes
		tag := quickfix.Tag(gFieldDef.Tag())
		err := fm.GetField(tag, &value)
		if err != nil {
			continue
		}

		fieldDesc, found := b.dictionary.FieldTypeByTag[int(tag)]
		if found {
			switch fieldDesc.Type {
			case "NUMINGROUP":
				res += b.BeautifyField(tag, fieldDesc.Name(), value, "", prefix, index) // ToDo: array = ${component}

				var getChildFieldTags func(parent quickfix.Tag, sFieldDefs FieldDefs) quickfix.GroupTemplate
				getChildFieldTags = func(parent quickfix.Tag, sFieldDefs FieldDefs) quickfix.GroupTemplate {
					template := quickfix.GroupTemplate{}

					groupField := sFieldDefs.Find(parent)
					if groupField == nil {
						fmt.Printf("ERROR GROUP[%d]\n", tag)
						return template
					}

					for _, subField := range groupField.Fields {
						fieldDesc, _ := b.dictionary.FieldTypeByTag[subField.Tag()]
						if subField.Tag() == int(parent) {
							template = append(template, quickfix.GroupElement(quickfix.Tag(subField.Tag())))
						}
						if fieldDesc.Type == "NUMINGROUP" {
							template = append(template, quickfix.GroupElement(quickfix.Tag(subField.Tag())))
							subTemplate := getChildFieldTags(quickfix.Tag(subField.Tag()), NewFieldDefsFromArr(groupField.Fields))
							template = append(template, subTemplate...)
						} else {
							template = append(template, quickfix.GroupElement(quickfix.Tag(subField.Tag())))
						}
					}

					return template
				}
				template := getChildFieldTags(tag, fieldDefs)

				groupField := fieldDefs.Find(tag)
				if groupField == nil {
					fmt.Printf("ERROR GROUP[%d]\n", tag)
					continue
				}
				group := quickfix.NewRepeatingGroup(tag, template)
				err := fm.GetGroup(group)
				if err != nil {
					res += fmt.Sprintf("ERROR REPEATING GROUP[%d] of [%v]\n", tag, template)
					continue
				}

				for i := 0; i < group.Len(); i++ {
					g := group.Get(i)
					idx := i
					if group.Len() == 1 {
						idx = -1
					}
					res += b.BeautifyFieldMap(g.FieldMap, NewFieldDefsFromArr(groupField.Fields), fixMessagePart_Group, prefix+printPrefixInc, idx)
				}

			default:
				strValue := fieldDesc.Enums[string(value)]
				res += b.BeautifyField(tag, fieldDesc.Name(), value, strValue.Description, prefix, index)
			}
		} else {
			res += fmt.Sprintf("ERROR: TAG[%d]=VALUE[%s]\n", tag, value)
		}
		index = -1
	}

	return res
}

// log incoming fix message
func (b BeautyLog) OnIncoming(raw []byte) {
	b.Log.OnIncoming(b.BeautifyFIX(raw))
}

// log outgoing fix message
func (b BeautyLog) OnOutgoing(raw []byte) {
	b.Log.OnOutgoing(b.BeautifyFIX(raw))
}
