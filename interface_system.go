package ipmi

import (
	"fmt"
	"math/rand"
	"os"
	"unsafe"

	"github.com/bougou/go-ipmi/open"
)

// connectOpen try to initialize the client by open the device of linux ipmi driver.
func (c *Client) connectOpen(devnum int32) error {
	// try the following devices
	var (
		ipmiDev1 string = fmt.Sprintf("/dev/ipmi%d", devnum)
		ipmiDev2 string = fmt.Sprintf("/dev/ipmi/%d", devnum)
		ipmiDev3 string = fmt.Sprintf("/dev/ipmidev/%d", devnum)
	)

	c.Debugf("Using ipmi device %d\n", devnum)
	var file *os.File
	if f, err := os.OpenFile(ipmiDev1, os.O_RDWR, 0); err != nil {
		c.Debugf("can not open ipmi dev (%s), err: %s", ipmiDev1, err)
		if f, err := os.OpenFile(ipmiDev2, os.O_RDWR, 0); err == nil {
			c.Debugf("can not open ipmi dev (%s), err: %s", ipmiDev2, err)
			if f, err := os.OpenFile(ipmiDev3, os.O_RDWR, 0); err != nil {
				c.Debugf("can not open ipmi dev (%s), err: %s", ipmiDev3, err)
				return fmt.Errorf("can not open ipmi dev")
			} else {
				file = f
				c.Debugf("opened ipmi dev file: %v\n", ipmiDev3)
			}
		} else {
			file = f
			c.Debugf("opened ipmi dev file: %v\n", ipmiDev2)
		}
	} else {
		file = f
		c.Debugf("opened ipmi dev file: %v\n", ipmiDev1)
	}
	if file == nil {
		return fmt.Errorf("ipmi dev file not opened")
	}

	c.Debugf("opened ipmi dev file: %v, descriptor is: %d\n", file, file.Fd())
	// set opened ipmi dev file
	c.openipmi.file = file

	var receiveEvents uint32 = 1
	if err := open.IOCTL(c.openipmi.file.Fd(), open.IPMICTL_SET_GETS_EVENTS_CMD, uintptr(unsafe.Pointer(&receiveEvents))); err != nil {
		return fmt.Errorf("ioctl failed, cloud not enable event receiver, err: %s", err)
	}

	return nil
}

func (c *Client) exchangeOpen(request Request, response Response) error {
	if c.openipmi.targetAddr != 0 && c.openipmi.targetAddr != c.openipmi.myAddr {

	} else {
		// otherwise use system interface
		c.Debugf("Sending request [%s] (%#02x) to System Interface\n", request.Command().Name, request.Command().ID)

	}

	recv, err := c.openSendRequest(request)
	if err != nil {
		return fmt.Errorf("openSendRequest failed, err: %s", err)
	}

	c.DebugBytes("recv data", recv, 16)

	// recv[0] is cc
	if len(recv) < 1 {
		return fmt.Errorf("recv data at least contains one completion code byte")
	}

	ccode := recv[0]
	if ccode != 0x00 {
		return &ResponseError{
			completionCode: CompletionCode(ccode),
			description:    fmt.Sprintf("ipmiRes CompletaionCode (%#02x) is not normal: %s", ccode, StrCC(response, ccode)),
		}
	}

	var unpackData = []byte{}
	if len(recv) > 1 {
		unpackData = recv[1:]
	}

	if err := response.Unpack(unpackData); err != nil {
		return &ResponseError{
			completionCode: CompletionCode(recv[0]),
			description:    fmt.Sprintf("unpack response failed, err: %s", err),
		}
	}

	c.Debug("<< Commmand Response", response)
	return nil
}

func (c *Client) openSendRequest(request Request) ([]byte, error) {

	var dataPtr *byte

	cmdData := request.Pack()
	if len(cmdData) > 0 {
		dataPtr = &cmdData[0]
	}

	msg := &open.IPMI_MSG{
		NetFn:   uint8(request.Command().NetFn),
		Cmd:     uint8(request.Command().ID),
		Data:    dataPtr,
		DataLen: uint16(len(cmdData)),
	}

	addr := &open.IPMI_SYSTEM_INTERFACE_ADDR{
		AddrType: open.IPMI_SYSTEM_INTERFACE_ADDR_TYPE,
		Channel:  open.IPMI_BMC_CHANNEL,
	}

	req := &open.IPMI_REQ{
		Addr:    addr,
		AddrLen: int(unsafe.Sizeof(addr)),
		MsgID:   rand.Int63(),
		Msg:     *msg,
	}

	c.Debug("IPMI_REQ", req)
	return open.SendCommand(c.openipmi.file, req)
}