# pool

Golang 实现的连接池


## 功能：

- 连接池中连接类型为`interface{}`，使得更加通用
- 链接的最大空闲时间，超时的链接将关闭丢弃，可避免空闲时链接自动失效问题
- 使用channel处理池中的链接，高效

## 基本用法

```go

//build 创建连接的方法
build := func() (interface{}, error) { return net.Dial("tcp", "127.0.0.1:4000") }

//destroy 关闭链接的方法
destroy := func(v interface{}) error { return v.(net.Conn).Close() }

//创建一个连接池： 初始化5，最大链接30
poolConfig := &pool.PoolConfig{
	InitialCap: 5,
	MaxCap:     30,
	Factory:    build,
	Close:      destroy,
	//链接最大空闲时间，超过该时间的链接 将会关闭，可避免空闲时链接EOF，自动失效的问题
	IdleTimeout: 15 * time.Second,
}
p, err := pool.NewChannelPool(poolConfig)
if err != nil {
	fmt.Println("err=", err)
}

//从连接池中取得一个链接
v, err := p.Get()

//do something
//conn=v.(net.Conn)

//将链接放回连接池中
p.Put(v)

//释放连接池中的所有链接
p.Release()

//查看当前链接中的数量
current := p.Len()


```


## License

The MIT License (MIT) - see LICENSE for more details
