/*
-------------------------------------------------
   Author :       Zhang Fan
   date：         2019/8/11
   Description :
-------------------------------------------------
*/

package tcp

import (
    "context"
    "errors"
    "github.com/go-kit/kit/endpoint"
    "github.com/go-kit/kit/log"
    "github.com/go-kit/kit/transport"
    "github.com/zlyuancn/ztcp/client"
    "sync"
)

type Client struct {
    tc           *client.Client
    reqblocks    reqBlockMapping
    enc          EncodeRequestFunc
    dec          DecodeResponseFunc
    before       []ClientRequestFunc
    after        []ClientResponseFunc
    finalizer    []ClientFinalizerFunc
    errorHandler transport.ErrorHandler
    waitConnect  chan struct{}
    waitClose    chan struct{}
    mx           sync.Mutex
}

func NewClient(
    tc *client.Client,
    enc EncodeRequestFunc,
    dec DecodeResponseFunc,
    options ...ClientOption,
) *Client {
    c := &Client{
        tc:           tc,
        reqblocks:    make(reqBlockMapping, 10),
        enc:          enc,
        dec:          dec,
        errorHandler: transport.NewLogErrorHandler(log.NewNopLogger()),
        waitConnect:  make(chan struct{}, 1),
        waitClose:    make(chan struct{}, 1),
    }
    for _, o := range options {
        o(c)
    }
    tc.Options().ClientGetDataObserves = append(tc.Options().ClientGetDataObserves, c.getConnData)
    tc.Options().ClientConnectObserves = append(tc.Options().ClientConnectObserves, c.connConnect)
    tc.Options().ClientCloseObserves = append(tc.Options().ClientCloseObserves, c.closeConnect)
    return c
}

type reqBlockMapping map[uint64]chan *rawResponse
type ClientOption func(c *Client)

// 在请求编码之前, 在请求对象上执行函数。
func ClientBefore(before ...ClientRequestFunc) ClientOption {
    return func(c *Client) { c.before = append(c.before, before...) }
}

// 在调用端点之后, 在响应解码之前执行ClientAfter
func ClientAfter(after ...ClientResponseFunc) ClientOption {
    return func(c *Client) { c.after = append(c.after, after...) }
}

// 用于处理非终端错误, 默认情况下非终端错误将被忽略
func ClientErrorHandler(errorHandler transport.ErrorHandler) ClientOption {
    return func(s *Client) { s.errorHandler = errorHandler }
}

// ClientFinalizer在每个请求的末尾执行
func ClientFinalizer(f ...ClientFinalizerFunc) ClientOption {
    return func(c *Client) { c.finalizer = append(c.finalizer, f...) }
}

type ClientFinalizerFunc func(ctx context.Context, err error)

func (m *Client) connConnect(c *client.Client) {
    m.waitConnect <- struct{}{}
}

func (m *Client) closeConnect(c *client.Client, err error) {
    m.waitClose <- struct{}{}
    m.mx.Lock()
    defer m.mx.Unlock()
    for id, reqblock := range m.reqblocks {
        reqblock <- newRawResponse(id, errors.New("连接已断开"), nil)
    }
    m.reqblocks = make(reqBlockMapping, 10)
}

func (m *Client) getConnData(c *client.Client, data []byte) {
    response := new(rawResponse)
    err := response.Load(data)
    if err == nil {
        err = response.err
    }
    if response.id != 0 {
        var reqblock chan *rawResponse
        var ok bool
        func() {
            m.mx.Lock()
            defer m.mx.Unlock()
            reqblock, ok = m.reqblocks[response.id]
            delete(m.reqblocks, response.id)
        }()
        if ok {
            reqblock <- response
        }
    }
}

func (m *Client) rpc(ctx context.Context, rpcName string, req []byte) ([]byte, error) {
    request := newRawRequest(AutoNextNum(), rpcName, req)
    data := request.Dump()

    reqblock := make(chan *rawResponse, 1)
    func() {
        m.mx.Lock()
        defer m.mx.Unlock()
        m.reqblocks[request.id] = reqblock
    }()

    if err := m.tc.Send(data); err != nil {
        m.errorHandler.Handle(ctx, err)
        return nil, err
    }

    response := <-reqblock
    if response.err != nil {
        m.errorHandler.Handle(ctx, response.err)
        return nil, response.err
    }
    return response.body, nil
}

func (m *Client) Endpoint(rpcName string) endpoint.Endpoint {
    return func(ctx context.Context, req interface{}) (interface{}, error) {
        var err error

        if len(m.finalizer) > 0 {
            defer func() {
                for _, f := range m.finalizer {
                    f(ctx, err)
                }
            }()
        }

        for _, f := range m.before {
            ctx = f(ctx, req)
        }

        var (
            request  []byte
            response []byte
        )

        request, err = m.enc(ctx, req)
        if err != nil {
            m.errorHandler.Handle(ctx, err)
            return nil, err
        }

        response, err = m.rpc(ctx, rpcName, request)
        if err != nil {
            m.errorHandler.Handle(ctx, err)
            return nil, err
        }

        for _, f := range m.after {
            ctx = f(ctx, response)
        }

        out, err := m.dec(ctx, response)
        if err != nil {
            m.errorHandler.Handle(ctx, err)
            return nil, err
        }

        return out, nil
    }
}

func (m *Client) WaitConnect() {
    if m.tc.IsConnected() {
        return
    }
    <-m.waitConnect
}

func (m *Client) WaitClose() {
    if m.tc.IsClosed() {
        return
    }
    <-m.waitClose
}
