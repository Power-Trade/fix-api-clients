package fix

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/quickfixgo/field"
	"github.com/quickfixgo/quickfix"
	"github.com/quickfixgo/quickfix/datadictionary"
	"golang.org/x/exp/maps"
)

type BeautyLogFactory struct {
	parent quickfix.LogFactory
}

const (
	fixMessagePart_Header  = 0
	fixMessagePart_Body    = 1
	fixMessagePart_Trailer = 2
	fixMessagePart_Group   = 3

	printPrefix    = 32
	printPrefixInc = 4

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

	emptyMap := func() *map[int]bool {
		m := make(map[int]bool)
		return &m
	}

	humanReadableFIX := b.BeautifyFIXString(string(raw))

	// Ideal result using Java: https://stackoverflow.com/questions/6453879/how-to-log-quickfix-message-in-human-readable-format
	return []byte(fmt.Sprintf("\nORIG:\n%s\n\nHEADER:\n%s\nBODY:\n%s\nTRAILER:\n%s\n",
		humanReadableFIX,
		b.BeautifyFieldMap(msg.Header.FieldMap, NewFieldDefs(b.dictionary.Header.Fields), fixMessagePart_Header, printPrefix, emptyMap()),
		b.BeautifyFieldMap(msg.Body.FieldMap, NewFieldDefs(MessageDesc.Fields), fixMessagePart_Body, printPrefix, emptyMap()),
		b.BeautifyFieldMap(msg.Trailer.FieldMap, NewFieldDefs(b.dictionary.Trailer.Fields), fixMessagePart_Trailer, printPrefix, emptyMap()),
	))
}

func (b BeautyLog) BeautifyFIXString(event string) string {
	return strings.ReplaceAll(event, "\x01", "|")
}

// tagOrder true if tag i should occur before tag j
type tagOrder func(i, j quickfix.Tag) bool

// ascending tags
func normalFieldOrder(i, j quickfix.Tag) bool { return i < j }

type tagSort struct {
	tags    []quickfix.Tag
	compare tagOrder
}

func (t tagSort) Len() int           { return len(t.tags) }
func (t tagSort) Swap(i, j int)      { t.tags[i], t.tags[j] = t.tags[j], t.tags[i] }
func (t tagSort) Less(i, j int) bool { return t.compare(t.tags[i], t.tags[j]) }

type FieldDefs []*datadictionary.FieldDef

func (t FieldDefs) Find(tag int) *datadictionary.FieldDef {
	for _, fieldDef := range t {
		if fieldDef.Tag() == tag {
			return fieldDef
		}
	}
	return nil
}

func NewFieldDefs(MapFields map[int]*datadictionary.FieldDef) FieldDefs {
	return maps.Values(MapFields)
}

func (b BeautyLog) BeautifyField(tag quickfix.Tag, name string, value quickfix.FIXBytes, desc string, prefix int) string {
	return fmt.Sprintf(
		"\t[%4d]\t%*s: %s [%s]\n",
		tag,
		prefix,
		name,
		value,
		desc,
	)
}

func (b BeautyLog) BeautifyFieldMap(fm quickfix.FieldMap, fieldDefs FieldDefs, messagePart int, prefix int, ignoreTags *map[int]bool) string {

	var res string
	if messagePart != fixMessagePart_Group {
		res = "--------------------------------------------------------------------\n"
	}

	sortedTags := tagSort{
		tags:    fm.Tags(),
		compare: normalFieldOrder,
	}

	sort.Sort(sortedTags)

	tagsInComponents := make(map[int]bool)

	for _, tag := range sortedTags.tags {

		var value quickfix.FIXBytes

		fm.GetField(tag, &value)

		fieldDesc, found := b.dictionary.FieldTypeByTag[int(tag)]
		if found {
			switch fieldDesc.Type {
			case "NUMINGROUP":

				res += b.BeautifyField(tag, fieldDesc.Name(), value, "", prefix)

				template := quickfix.GroupTemplate{}
				groupField := fieldDefs.Find(int(tag))
				if groupField == nil {
					res += fmt.Sprintf("ERROR GROUP[%d]", tag)
					continue
				}

				for _, groupField := range groupField.Fields {
					template = append(template, quickfix.GroupElement(quickfix.Tag(groupField.Tag())))
				}

				group := quickfix.NewRepeatingGroup(tag, template)
				err := fm.GetGroup(group)
				if err != nil {
					res += fmt.Sprintf("ERROR GROUP[%d]", tag)
					continue
				}

				for i := 0; i < group.Len(); i++ {
					g := group.Get(i)
					res += b.BeautifyFieldMap(g.FieldMap, groupField.Fields, fixMessagePart_Group, prefix+printPrefixInc, &tagsInComponents)
				}

			default:
				if _, ok := tagsInComponents[fieldDesc.Tag()]; ok {
					// Tag is a member of a child's component
					// It was already printed as a part of the component
					continue
				}
				(*ignoreTags)[fieldDesc.Tag()] = true
				strValue := fieldDesc.Enums[string(value)]
				res += b.BeautifyField(tag, fieldDesc.Name(), value, strValue.Description, prefix)
			}
		} else {
			res += fmt.Sprintf("ERROR: TAG[%d]=VALUE[%s]\n", tag, value)
		}
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
