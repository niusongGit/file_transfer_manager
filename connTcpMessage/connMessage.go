package connTcpMessage

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

const Package_max_size uint32 = 1024 * 1024 * 1024

type ConnTcp struct {
	Host   string
	Port   int
	Router *Router
}

func NewConnTcp(host string, port int) *ConnTcp {

	//注册回复消息Router
	r := NewRouter()
	r.lock.Lock()
	r.handlersMapping[0] = Recv_rev
	r.lock.Unlock()

	return &ConnTcp{
		Host:   host,
		Port:   port,
		Router: r,
	}
}

func (c *ConnTcp) StartUP() {
	listener, err := net.Listen("tcp", c.GetSelfAddr())
	if err != nil {
		panic(err)
	}
	log.Println("Listening... ...")

	for {
		conn, err := listener.Accept()
		if err != nil {
			panic(err)
		}
		log.Println("connect success")
		go c.handFunc(conn)
	}
}

func (c *ConnTcp) handFunc(conn net.Conn) {
	defer conn.Close()
	//这儿循环读才能续用conn
	for {
		//读取
		recv, err := c.recvPackage(conn)
		if err != nil {
			log.Println(err)
			break
		}

		fun := c.Router.GetRouter(recv.MID)
		send, err := fun(context.Background(), recv)
		//回复
		var errNote string
		if err != nil {
			errNote = err.Error()
		}
		err = c.sendPackage(conn, 0, send, errNote)
		if err != nil {
			log.Println(err)
			break
		}
		fmt.Println("服务端：", conn.LocalAddr(), "客户端", conn.RemoteAddr())
	}

}

func (c *ConnTcp) Send(conn net.Conn, addr string, mid uint64, msg []byte, errNote string, timeout time.Duration) ([]byte, bool, error) {
	var err error
	//一次性conn
	if conn == nil {
		conn, err = net.DialTimeout("tcp", addr, timeout)
		if err != nil {
			return nil, false, err
		}
		defer conn.Close()
	}

	err = conn.SetDeadline(time.Now().Add(timeout))
	if err != nil {
		return nil, false, err
	}
	defer conn.SetDeadline(time.Time{})

	fmt.Println("客户端：", conn.LocalAddr(), "服务端", conn.RemoteAddr())
	err = c.sendPackage(conn, mid, msg, errNote)
	if err != nil {
		return nil, false, err
	}
	recv, err := c.recvPackage(conn)
	if err != nil {
		return nil, false, err
	}

	fun := c.Router.GetRouter(recv.MID)
	data, err := fun(context.Background(), recv)
	return data, true, err
}

func (c *ConnTcp) sendPackage(conn net.Conn, mid uint64, msg []byte, errNote string) error {
	message, err := NewMessage(mid, c.GetSelfAddr(), conn.RemoteAddr().String(), msg, errNote).SerializationMessage()
	if err != nil {
		return err
	}

	// 计算数据包的长度
	length := uint32(len(message))

	if length > Package_max_size {
		return errors.New("packet size too big!!")
	}

	// 构造数据包
	packet := make([]byte, 4+length)
	binary.BigEndian.PutUint32(packet, length)
	copy(packet[4:], message)

	_, err = conn.Write(packet)
	if err != nil {
		return err
	}
	return nil
}

func (c *ConnTcp) recvPackage(conn net.Conn) (*Message, error) {
	// 读取数据包的头部，指示数据包的长度
	header := make([]byte, 4)
	_, err := io.ReadFull(conn, header)
	if err != nil {
		return nil, err
	}

	// 解析数据包的长度
	length := binary.BigEndian.Uint32(header)

	if length > Package_max_size {
		return nil, errors.New("packet size too big!!")
	}
	// 读取数据包
	data := make([]byte, length)
	_, err = io.ReadFull(conn, data)
	if err != nil {
		return nil, err
	}
	message, err := (&Message{}).ParseMessage(data)
	if err != nil {
		return nil, err
	}
	return message, nil
}

func (c *ConnTcp) GetSelfAddr() string {
	if c.Port == 0 || c.Host == "" {
		return ""
	}
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
