package channel

import (
	"bytes"
	"encoding/binary"
	"errors"

	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type Message struct {
	Length      uint32
	Type        uint16
	Seq         string
	Result      int32
	TopicLength byte
	Topic       string
	Data        []byte
}

func NewMessage(typ int, topic string, data interface{}) (*Message, error) {
	d, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	msg := &Message{
		Length:      uint32(42 + len(d)),
		Type:        uint16(typ),
		Seq:         newSeq(),
		TopicLength: byte(len(topic) + 1),
		Topic:       topic,
		Result:      0,
		Data:        d,
	}
	if typ == TypeRegisterEventLog {
		msg.Length += uint32(1 + len(topic))
	}
	return msg, nil
}

func DecodeMessage(data []byte) (msg Message, err error) {
	buf := bytes.NewBuffer(data)
	if err = binary.Read(buf, binary.BigEndian, &msg.Length); err != nil {
		return
	}
	if uint32(len(data)) < msg.Length {
		return msg, errors.New("uncomplete message")
	}
	if err = binary.Read(buf, binary.BigEndian, &msg.Type); err != nil {
		return
	}
	var seq [32]byte
	if err = binary.Read(buf, binary.LittleEndian, &seq); err != nil {
		return
	}
	msg.Seq = string(seq[:])
	if err = binary.Read(buf, binary.LittleEndian, &msg.Result); err != nil {
		return
	}
	// tlen, err := buf.ReadByte()
	// if err != nil {
	// 	return
	// }
	// var topic []byte
	// if tlen > 1 {
	// 	topic = make([]byte, tlen-1)
	// 	if err = binary.Read(buf, binary.LittleEndian, &topic); err != nil {
	// 		return
	// 	}
	// }
	// msg.TopicLength = tlen
	// msg.Topic = string(topic)
	msg.Data = buf.Bytes()
	return
}

func (msg *Message) Encode() []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, msg.Length)
	binary.Write(buf, binary.BigEndian, msg.Type)
	binary.Write(buf, binary.LittleEndian, []byte(msg.Seq))
	binary.Write(buf, binary.BigEndian, msg.Result)
	if msg.Type == TypeRegisterEventLog {
		buf.WriteByte(msg.TopicLength)
		buf.WriteString(msg.Topic)
	}
	binary.Write(buf, binary.LittleEndian, msg.Data)
	return buf.Bytes()
}
