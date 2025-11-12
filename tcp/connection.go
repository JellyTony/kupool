package tcp

import (
	"io"
	"net"

	kupool "github.com/JellyTony/kupool"
	"github.com/JellyTony/kupool/wire/endian"
)

// Frame Frame
type Frame struct {
	OpCode  kupool.OpCode
	Payload []byte
}

// SetOpCode SetOpCode
func (f *Frame) SetOpCode(code kupool.OpCode) {
	f.OpCode = code
}

// GetOpCode GetOpCode
func (f *Frame) GetOpCode() kupool.OpCode {
	return f.OpCode
}

// SetPayload SetPayload
func (f *Frame) SetPayload(payload []byte) {
	f.Payload = payload
}

// GetPayload GetPayload
func (f *Frame) GetPayload() []byte {
	return f.Payload
}

// Conn Conn
type TcpConn struct {
	net.Conn
}

// NewConn NewConn
func NewConn(conn net.Conn) *TcpConn {
	return &TcpConn{
		Conn: conn,
	}
}

// ReadFrame ReadFrame
func (c *TcpConn) ReadFrame() (kupool.Frame, error) {
	opcode, err := endian.ReadUint8(c.Conn)
	if err != nil {
		return nil, err
	}
	payload, err := endian.ReadBytes(c.Conn)
	if err != nil {
		return nil, err
	}
	return &Frame{
		OpCode:  kupool.OpCode(opcode),
		Payload: payload,
	}, nil
}

// WriteFrame WriteFrame
func (c *TcpConn) WriteFrame(code kupool.OpCode, payload []byte) error {
	return WriteFrame(c.Conn, code, payload)
}

// Flush Flush
func (c *TcpConn) Flush() error {
	return nil
}

// WriteFrame write a frame to w
func WriteFrame(w io.Writer, code kupool.OpCode, payload []byte) error {
	if err := endian.WriteUint8(w, uint8(code)); err != nil {
		return err
	}
	if err := endian.WriteBytes(w, payload); err != nil {
		return err
	}
	return nil
}
