package eiscp

import (
	"fmt"
	"github.com/op/go-logging"
	"net"
	"strings"
	"time"
)

var log = logging.MustGetLogger(`eiscp`)

const DEFAULT_DISCOVERY_TIMEOUT = time.Duration(5) * time.Second
const DEFAULT_RESPONSE_TIMEOUT = time.Duration(3) * time.Second
const DEFAULT_DISCOVERY_PORT = 60128

func Discover(timeout time.Duration, discoveryRange string) ([]*Device, error) {
	d := NewDiscoverer(timeout)

	if discoveryRange != `` && discoveryRange != `auto` {
		if strings.Contains(discoveryRange, `/`) {
			if ip, _, err := net.ParseCIDR(discoveryRange); err == nil {
				d.discoveryRange = &net.UDPAddr{
					IP:   ip,
					Port: DEFAULT_DISCOVERY_PORT,
				}
			} else {
				return nil, err
			}
		} else {
			if ip := net.ParseIP(discoveryRange); ip != nil {
				d.FirstOnly = true
				d.discoveryRange = &net.UDPAddr{
					IP:   ip,
					Port: DEFAULT_DISCOVERY_PORT,
				}
			} else {
				return nil, fmt.Errorf("Invalid IP specified %q", discoveryRange)
			}
		}

	}

	return d.Perform()
}

type Discoverer struct {
	Timeout        time.Duration
	FirstOnly      bool
	listenAddr     *net.UDPAddr
	discoveryRange *net.UDPAddr
}

func NewDiscoverer(timeout ...time.Duration) *Discoverer {
	tout := DEFAULT_DISCOVERY_TIMEOUT

	if len(timeout) == 1 {
		tout = timeout[0]
	}

	return &Discoverer{
		Timeout: tout,
		listenAddr: &net.UDPAddr{
			IP:   net.IPv4zero,
			Port: 0,
		},
		discoveryRange: &net.UDPAddr{
			IP:   net.IPv4bcast,
			Port: DEFAULT_DISCOVERY_PORT,
		},
	}
}

func (self *Discoverer) Perform() ([]*Device, error) {
	devices := make([]*Device, 0)

	if conn, err := net.ListenUDP(`udp`, self.listenAddr); err == nil {
		discoverPacket := encodePacket(`ECNQSTN`, CategoryAny)
		log.Debugf("Sending discovery packet: %s", discoverPacket.debug())

		// write discovery packet
		if _, err = conn.WriteToUDP(discoverPacket.bytes(), self.discoveryRange); err == nil {
			data := make([]byte, maxPacketSize)
			errors := make(chan error)

			go func() {
				for {
					if msglen, from, err := conn.ReadFromUDP(data); err == nil {
						// create a device from any response packets that aren't the reflected discovery packet
						if responsePacket := packet(data[:msglen]); !responsePacket.equals(discoverPacket) {
							if device, err := self.createDeviceFromResponse(responsePacket, from); err == nil {
								devices = append(devices, device)

								if self.FirstOnly {
									errors <- nil
									return
								}
							} else {
								errors <- err
								return
							}
						}
					} else {
						errors <- err
						return
					}
				}
			}()

			select {
			case err := <-errors:
				return devices, err
			case <-time.After(self.Timeout):
				break
			}

			return devices, nil
		} else {
			return devices, err
		}
	} else {
		return devices, err
	}
}

func (self *Discoverer) createDeviceFromResponse(responsePacket packet, from *net.UDPAddr) (*Device, error) {
	var info DeviceInfo

	if err := responsePacket.parseInfo(&info); err == nil {
		if device, err := NewDevice(from, info); err == nil {
			return device, nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}
