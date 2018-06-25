package pool

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

//PoolConfig 连接池相关配置
type TPoolConf struct {
	//连接池中拥有的最小连接数
	InitialCap int
	//连接池中拥有的最大的连接数
	MaxCap int
	//生成连接的方法
	Factory func() (interface{}, error)
	//关闭链接的方法
	Close func(interface{}) error
	//链接最大空闲时间，超过该事件则将失效
	IdleTimeout time.Duration
}

//channelPool 存放链接信息
type channelPool struct {
	mu            sync.Mutex
	cons          chan *idleConn
	factory       func() (interface{}, error)
	close         func(interface{}) error
	idleTimeout   time.Duration
}

type idleConn struct {
	conn      interface{}
	t         time.Time
}

//NewChannelPool 初始化链接
func NewChannelPool(poolConfig *TPoolConf) (Pool, error) {
	if poolConfig.InitialCap < 0 || poolConfig.MaxCap <= 0 || poolConfig.InitialCap > poolConfig.MaxCap {
		return nil, errors.New("invalid capacity settings")
	}

	c := &channelPool{
		cons:       make(chan *idleConn, poolConfig.MaxCap),
		factory:     poolConfig.Factory,
		close:       poolConfig.Close,
		idleTimeout: poolConfig.IdleTimeout,
	}

	for i := 0; i < poolConfig.InitialCap; i++ {
		conn, err := c.factory()
		if err != nil {
			c.Release()
			return nil, fmt.Errorf("factory is not able to fill the pool: %s", err)
		}
		c.cons <- &idleConn{conn: conn, t: time.Now()}
	}

	return c, nil
}

//getConnS 获取所有连接
func (c *channelPool) getConnS() chan *idleConn {
	c.mu.Lock()
	cons := c.cons
	c.mu.Unlock()
	return cons
}

//Get 从pool中取一个连接
func (c *channelPool) Get() (interface{}, error) {
	cons := c.getConnS()
	if cons == nil {
		return nil, ErrClosed
	}
	for {
		select {
		case wrapConn := <-cons:
			if wrapConn == nil {
				return nil, ErrClosed
			}
			//判断是否超时，超时则丢弃
			if timeout := c.idleTimeout; timeout > 0 {
				if wrapConn.t.Add(timeout).Before(time.Now()) {
					//丢弃并关闭该链接
					c.Close(wrapConn.conn)
					continue
				}
			}
			return wrapConn.conn, nil
		default:
			conn, err := c.factory()
			if err != nil {
				return nil, err
			}

			return conn, nil
		}
	}
}

//Put 将连接放回pool中
func (c *channelPool) Put(conn interface{}) error {
	if conn == nil {
		return errors.New("connection is nil. rejecting")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cons == nil {
		return c.Close(conn)
	}

	select {
	case c.cons <- &idleConn{conn: conn, t: time.Now()}:
		return nil
	default:
		//连接池已满，直接关闭该链接
		return c.Close(conn)
	}
}

//Close 关闭单条连接
func (c *channelPool) Close(conn interface{}) error {
	if conn == nil {
		return errors.New("connection is nil. rejecting")
	}
	return c.close(conn)
}

//Release 释放连接池中所有链接
func (c *channelPool) Release() {
	c.mu.Lock()
	cons := c.cons
	c.cons = nil
	c.factory = nil
	closeFun := c.close
	c.close = nil
	c.mu.Unlock()

	if cons == nil {
		return
	}

	close(cons)
	for wrapConn := range cons {
		closeFun(wrapConn.conn)
	}
}

//Len 连接池中已有的连接
func (c *channelPool) Len() int {
	return len(c.getConnS())
}
