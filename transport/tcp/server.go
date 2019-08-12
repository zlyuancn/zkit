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
    "fmt"
    "github.com/go-kit/kit/endpoint"
    "github.com/go-kit/kit/log"
    "github.com/go-kit/kit/transport"
    "github.com/zlyuancn/ztcp/client"
    "github.com/zlyuancn/ztcp/server"
)

type Server struct {
    ts           *server.Server
    rpcs         rpcMapping
    dec          DecodeRequestFunc
    enc          EncodeResponseFunc
    before       []ServerRequestFunc
    after        []ServerResponseFunc
    finalizer    []ServerFinalizerFunc
    errorHandler transport.ErrorHandler
    done         chan struct{}
}

func NewServer(
    ts *server.Server,
    dec DecodeRequestFunc,
    enc EncodeResponseFunc,
    options ...ServerOption,
) *Server {
    s := &Server{
        ts:           ts,
        rpcs:         make(rpcMapping),
        dec:          dec,
        enc:          enc,
        errorHandler: transport.NewLogErrorHandler(log.NewNopLogger()),
        done:         make(chan struct{}, 1),
    }
    for _, o := range options {
        o(s)
    }
    ts.Options().ClientGetDataObserves = append(ts.Options().ClientGetDataObserves, s.getConnData)
    return s
}

type rpcMapping map[string]endpoint.Endpoint
type ServerOption func(s *Server)

// 在请求解码之前, 在请求对象上执行函数。
func ServerBefore(before ...ServerRequestFunc) ServerOption {
    return func(s *Server) { s.before = append(s.before, before...) }
}

// 在调用端点之后, 在响应编码之前执行ServerAfter
func ServerAfter(after ...ServerResponseFunc) ServerOption {
    return func(s *Server) { s.after = append(s.after, after...) }
}

// 用于处理非终端错误, 默认情况下非终端错误将被忽略
func ServerErrorHandler(errorHandler transport.ErrorHandler) ServerOption {
    return func(s *Server) { s.errorHandler = errorHandler }
}

// ServerFinalizer在每个请求的末尾执行
func ServerFinalizer(f ...ServerFinalizerFunc) ServerOption {
    return func(s *Server) { s.finalizer = append(s.finalizer, f...) }
}

type ServerFinalizerFunc func(ctx context.Context, err error)

func (m *Server) getConnData(c *client.Client, data []byte) {
    var err error
    var req = new(rawRequest)
    var resp *rawResponse

    err = req.Load(data)
    if err == nil {
        _, resp, err = m.rpc(context.Background(), req)
    }

    if err != nil {
        resp = newRawResponse(req.id, err, nil)
    }

    _ = c.Send(resp.Dump())
}

func (m *Server) rpc(ctx context.Context, req *rawRequest) (context.Context, *rawResponse, error) {
    var err error

    if len(m.finalizer) > 0 {
        defer func() {
            for _, f := range m.finalizer {
                f(ctx, err)
            }
        }()
    }

    for _, f := range m.before {
        ctx = f(ctx, req.body)
    }

    var (
        request  interface{}
        response interface{}
    )

    request, err = m.dec(ctx, req.body)
    if err != nil {
        m.errorHandler.Handle(ctx, err)
        return ctx, nil, err
    }

    ep, ok := m.rpcs[req.rpc]
    if !ok {
        err = errors.New(fmt.Sprintf("服务 %s 不存在", req.rpc))
        m.errorHandler.Handle(ctx, err)
        return ctx, nil, err
    }

    response, err = ep(ctx, request)
    if err != nil {
        m.errorHandler.Handle(ctx, err)
        return ctx, nil, err
    }

    for _, f := range m.after {
        ctx = f(ctx, response)
    }

    out, err := m.enc(ctx, response)
    if err != nil {
        m.errorHandler.Handle(ctx, err)
        return ctx, nil, err
    }

    resp := newRawResponse(req.id, nil, out)
    return ctx, resp, err
}

func (m *Server) RegRpc(rpcName string, ep endpoint.Endpoint) {
    if rpcName == "" {
        panic(errors.New("空的rpcName"))
    }
    m.rpcs[rpcName] = ep
}
func (m *Server) UnRegRpc(rpcName string) {
    if rpcName != "" {
        delete(m.rpcs, rpcName)
    }
}

// 执行这个方法会阻塞当前 goroutine, 直到调用 Stop 为止
func (m *Server) Run() {
    <-m.done
}

// 结束 Run 的阻塞
func (m *Server) Stop() {
    m.done <- struct{}{}
}
