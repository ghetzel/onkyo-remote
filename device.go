package eiscp

import (
	"net"
	"runtime"
	"strings"
)

type IDevice interface {
	Info() DeviceInfo
	Address() net.Addr
	Messages() <-chan Message
	Send(cmd string, params ...string) error
}

type DeviceInfo struct {
	Model      string
	Category   DeviceCategory
	DestArea   string
	Identifier string
	Port       int
}

type DeviceCategory byte

func (d DeviceCategory) String() string {
	return string(d)
}

const (
	CategoryDevice = DeviceCategory('1') // Denotes receivers.
	CategoryAny    = DeviceCategory('x') // Used for discovery
)

type Device struct {
	IDevice
	conn   net.Conn
	info   DeviceInfo
	recv   chan Message
	remote net.Addr
}

func NewDevice(addr net.Addr, info DeviceInfo) (*Device, error) {
	if conn, err := net.Dial(`tcp`, addr.String()); err == nil {
		d := &Device{
			conn:   conn,
			info:   info,
			recv:   make(chan Message),
			remote: addr,
		}

		go d.listen()

		return d, nil
	} else {
		return nil, err
	}
}

func (self *Device) Info() DeviceInfo {
	return self.info
}

func (self *Device) Address() net.Addr {
	return self.remote
}

func (self *Device) Messages() <-chan Message {
	return self.recv
}

func (self *Device) Send(cmd string, params ...string) error {
	cmdstring := cmd + strings.Join(params, ``)
	packet := encodePacket(cmdstring, self.info.Category)
	_, err := self.conn.Write(packet.bytes())
	return err
}

func (self *Device) listen() {
	runtime.SetFinalizer(self, func(self *Device) {
		self.conn.Close()
	})

	data := make([]byte, maxPacketSize)

	for {
		if datalen, err := self.conn.Read(data); err == nil {
			packets, err := decodePackets(data[:datalen])

			if err != nil {
				log.Warningf("Failed to decode packet: %v", err)
			}

			for _, pkt := range packets {
				message := pkt.Message()

				switch message.Code() {
				case `NLT`, `NLS`:
					continue
				default:
					self.recv <- pkt.Message()
				}
			}
		} else {
			log.Errorf("Failed to read response: %v", err)
			break
		}
	}

	runtime.SetFinalizer(self, nil)
	self.conn.Close()
	close(self.recv)
}
